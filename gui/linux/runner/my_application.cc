#include "my_application.h"

#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <flutter_linux/flutter_linux.h>
#include <netinet/in.h>
#include <signal.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

#include <chrono>
#include <cctype>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>
#include <utility>
#include <vector>

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif

#include "flutter/generated_plugin_registrant.h"

namespace {

struct BundledRuntime {
  std::filesystem::path daemon;
  std::filesystem::path caddy;
};

struct DaemonConnection {
  std::string endpoint;
  std::string token;
};

pid_t owned_daemon_pid = -1;

std::string TrimWhitespace(std::string value) {
  while (!value.empty() && std::isspace(static_cast<unsigned char>(value.back()))) {
    value.pop_back();
  }
  size_t first = 0;
  while (first < value.size() &&
         std::isspace(static_cast<unsigned char>(value[first]))) {
    ++first;
  }
  return value.substr(first);
}

bool IsRegularFile(const std::filesystem::path& path) {
  std::error_code error;
  return std::filesystem::is_regular_file(path, error);
}

std::filesystem::path CurrentExecutablePath() {
  std::vector<char> buffer(4096);
  for (;;) {
    const ssize_t length = readlink("/proc/self/exe", buffer.data(), buffer.size() - 1);
    if (length < 0) {
      return {};
    }
    if (static_cast<size_t>(length) < buffer.size() - 1) {
      buffer[static_cast<size_t>(length)] = '\0';
      return std::filesystem::path(buffer.data());
    }
    if (buffer.size() >= 32768) {
      return {};
    }
    buffer.resize(buffer.size() * 2);
  }
}

std::optional<BundledRuntime> FindBundledRuntime() {
  const auto executable = CurrentExecutablePath();
  if (executable.empty()) {
    return std::nullopt;
  }

  auto directory = executable.parent_path();
  for (int level = 0; level < 8 && !directory.empty(); ++level) {
    const auto daemon = directory / "bin" / "davd";
    const auto caddy = directory / "libexec" / "caddy";
    if (IsRegularFile(daemon) && IsRegularFile(caddy)) {
      return BundledRuntime{daemon, caddy};
    }
    const auto parent = directory.parent_path();
    if (parent == directory) {
      break;
    }
    directory = parent;
  }
  return std::nullopt;
}

std::optional<std::string> ReadFile(const std::filesystem::path& path) {
  std::ifstream file(path, std::ios::binary);
  if (!file) {
    return std::nullopt;
  }
  const std::string content((std::istreambuf_iterator<char>(file)),
                            std::istreambuf_iterator<char>());
  const std::string result = TrimWhitespace(content);
  return result.empty() ? std::nullopt
                        : std::optional<std::string>(result);
}

std::optional<std::string> Environment(const char* name) {
  const char* value = std::getenv(name);
  if (value == nullptr || *value == '\0') {
    return std::nullopt;
  }
  return std::string(value);
}

std::vector<std::pair<std::filesystem::path, std::filesystem::path>>
DaemonConnectionPaths() {
  std::vector<std::pair<std::filesystem::path, std::filesystem::path>> paths;
  const auto endpoint_override = Environment("DAVDECK_ENDPOINT");
  if (endpoint_override.has_value()) {
    const auto token_override = Environment("DAVDECK_TOKEN_FILE");
    const auto home = Environment("HOME");
    const auto config = Environment("XDG_CONFIG_HOME");
    std::filesystem::path token_path;
    if (token_override.has_value()) {
      token_path = *token_override;
    } else if (config.has_value()) {
      token_path = std::filesystem::path(*config) / "DavDeck" / "management.token";
    } else if (home.has_value()) {
      token_path = std::filesystem::path(*home) / ".config" / "DavDeck" / "management.token";
    }
    paths.emplace_back(std::filesystem::path(), token_path);
    return paths;
  }

  // Prefer an installed server daemon when one is present. This lets the GUI
  // act as a client of systemd without ever stopping that external daemon.
  paths.emplace_back("/run/davdeck/management.endpoint",
                     "/etc/davdeck/management.token");

  const auto home = Environment("HOME");
  if (!home.has_value()) {
    return paths;
  }
  const auto runtime = Environment("XDG_RUNTIME_DIR");
  const auto config = Environment("XDG_CONFIG_HOME");
  const std::filesystem::path endpoint_path =
      runtime.has_value()
          ? std::filesystem::path(*runtime) / "DavDeck" / "management.endpoint"
          : std::filesystem::path(*home) / ".cache" / "DavDeck" / "run" / "management.endpoint";
  const std::filesystem::path token_path =
      config.has_value()
          ? std::filesystem::path(*config) / "DavDeck" / "management.token"
          : std::filesystem::path(*home) / ".config" / "DavDeck" / "management.token";
  paths.emplace_back(endpoint_path, token_path);
  return paths;
}

std::optional<DaemonConnection> ReadDaemonConnection() {
  for (const auto& paths : DaemonConnectionPaths()) {
    std::optional<std::string> endpoint;
    if (paths.first.empty()) {
      endpoint = Environment("DAVDECK_ENDPOINT");
    } else {
      endpoint = ReadFile(paths.first);
    }
    const auto token = ReadFile(paths.second);
    if (endpoint.has_value() && token.has_value() && !token->empty()) {
      return DaemonConnection{*endpoint, *token};
    }
  }
  return std::nullopt;
}

bool ParseLoopbackEndpoint(const std::string& endpoint, sockaddr_storage* address,
                           socklen_t* address_length, std::string* host) {
  if (endpoint.rfind("http://", 0) != 0) {
    return false;
  }
  const std::string authority = endpoint.substr(7);
  if (authority.empty() || authority.find_first_of("/?#") != std::string::npos) {
    return false;
  }

  std::string host_value;
  std::string port_value;
  if (authority.front() == '[') {
    const size_t closing = authority.find(']');
    if (closing == std::string::npos || closing + 2 > authority.size() ||
        authority[closing + 1] != ':') {
      return false;
    }
    host_value = authority.substr(1, closing - 1);
    port_value = authority.substr(closing + 2);
  } else {
    const size_t separator = authority.rfind(':');
    if (separator == std::string::npos || authority.find(':') != separator) {
      return false;
    }
    host_value = authority.substr(0, separator);
    port_value = authority.substr(separator + 1);
  }
  if (host_value != "127.0.0.1" && host_value != "::1") {
    return false;
  }
  char* end = nullptr;
  const long parsed_port = std::strtol(port_value.c_str(), &end, 10);
  if (port_value.empty() || end == port_value.c_str() || *end != '\0' ||
      parsed_port < 1 || parsed_port > 65535) {
    return false;
  }

  std::memset(address, 0, sizeof(*address));
  if (host_value == "127.0.0.1") {
    auto* ipv4 = reinterpret_cast<sockaddr_in*>(address);
    ipv4->sin_family = AF_INET;
    ipv4->sin_port = htons(static_cast<uint16_t>(parsed_port));
    if (inet_pton(AF_INET, host_value.c_str(), &ipv4->sin_addr) != 1) {
      return false;
    }
    *address_length = sizeof(sockaddr_in);
  } else {
    auto* ipv6 = reinterpret_cast<sockaddr_in6*>(address);
    ipv6->sin6_family = AF_INET6;
    ipv6->sin6_port = htons(static_cast<uint16_t>(parsed_port));
    if (inet_pton(AF_INET6, host_value.c_str(), &ipv6->sin6_addr) != 1) {
      return false;
    }
    *address_length = sizeof(sockaddr_in6);
  }
  *host = host_value;
  return true;
}

bool RequestDaemon(const char* method, const char* path, int expected_status) {
  const auto connection = ReadDaemonConnection();
  if (!connection.has_value() ||
      connection->token.find_first_of("\r\n") != std::string::npos) {
    return false;
  }

  sockaddr_storage address{};
  socklen_t address_length = 0;
  std::string host;
  if (!ParseLoopbackEndpoint(connection->endpoint, &address, &address_length,
                             &host)) {
    return false;
  }
  const int socket_fd = socket(address.ss_family, SOCK_STREAM, 0);
  if (socket_fd < 0) {
    return false;
  }
  timeval timeout{};
  timeout.tv_sec = 1;
  setsockopt(socket_fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, sizeof(timeout));
  setsockopt(socket_fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, sizeof(timeout));
  if (connect(socket_fd, reinterpret_cast<sockaddr*>(&address), address_length) != 0) {
    close(socket_fd);
    return false;
  }

  const std::string request = std::string(method) + " " + path +
                              " HTTP/1.1\r\nHost: " + host +
                              "\r\nAuthorization: Bearer " + connection->token +
                              "\r\nConnection: close\r\n\r\n";
  size_t sent = 0;
  while (sent < request.size()) {
    const ssize_t count = send(socket_fd, request.data() + sent,
                               request.size() - sent, MSG_NOSIGNAL);
    if (count <= 0) {
      close(socket_fd);
      return false;
    }
    sent += static_cast<size_t>(count);
  }

  std::string response;
  char buffer[512];
  while (response.find("\r\n") == std::string::npos && response.size() < 4096) {
    const ssize_t count = recv(socket_fd, buffer, sizeof(buffer), 0);
    if (count <= 0) {
      break;
    }
    response.append(buffer, static_cast<size_t>(count));
  }
  close(socket_fd);
  const std::string status_marker = " " + std::to_string(expected_status) + " ";
  return response.rfind("HTTP/1.", 0) == 0 && response.find(status_marker) != std::string::npos;
}

bool DaemonIsReachable() {
  return RequestDaemon("GET", "/api/v1/status", 200);
}

bool RequestDaemonShutdown() {
  return RequestDaemon("POST", "/api/v1/daemon/shutdown", 200);
}

void RedirectDaemonOutput() {
  const auto home = Environment("HOME");
  if (!home.has_value()) {
    return;
  }
  const std::filesystem::path log_directory =
      std::filesystem::path(*home) / ".cache" / "DavDeck" / "logs";
  std::error_code error;
  std::filesystem::create_directories(log_directory, error);
  const int log_fd = open((log_directory / "davd.log").c_str(),
                          O_WRONLY | O_CREAT | O_APPEND, 0600);
  if (log_fd < 0) {
    return;
  }
  dup2(log_fd, STDOUT_FILENO);
  dup2(log_fd, STDERR_FILENO);
  close(log_fd);
}

void StartBundledDaemonIfNeeded() {
  if (DaemonIsReachable()) {
    return;
  }
  const auto runtime = FindBundledRuntime();
  if (!runtime.has_value()) {
    // Development builds and headless packages have no GUI runtime. The GUI
    // can still connect to a daemon started separately by the user/service.
    return;
  }

  const pid_t child = fork();
  if (child < 0) {
    return;
  }
  if (child == 0) {
    RedirectDaemonOutput();
    execl(runtime->daemon.c_str(), runtime->daemon.c_str(), "--caddy-binary",
          runtime->caddy.c_str(), "--portable-owner", "gui",
          static_cast<char*>(nullptr));
    _exit(127);
  }
  owned_daemon_pid = child;
  usleep(200000);
  int status = 0;
  if (waitpid(owned_daemon_pid, &status, WNOHANG) == owned_daemon_pid) {
    owned_daemon_pid = -1;
  }
}

void StopBundledDaemonIfOwned() {
  if (owned_daemon_pid <= 0) {
    return;
  }
  int status = 0;
  if (waitpid(owned_daemon_pid, &status, WNOHANG) == 0) {
    RequestDaemonShutdown();
    const auto deadline = std::chrono::steady_clock::now() +
                          std::chrono::seconds(5);
    while (std::chrono::steady_clock::now() < deadline) {
      if (waitpid(owned_daemon_pid, &status, WNOHANG) == owned_daemon_pid) {
        owned_daemon_pid = -1;
        return;
      }
      usleep(100000);
    }
    kill(owned_daemon_pid, SIGTERM);
    usleep(200000);
    if (waitpid(owned_daemon_pid, &status, WNOHANG) == 0) {
      kill(owned_daemon_pid, SIGKILL);
    }
  }
  waitpid(owned_daemon_pid, &status, 0);
  owned_daemon_pid = -1;
}

}  // namespace

struct _MyApplication {
  GtkApplication parent_instance;
  char** dart_entrypoint_arguments;
};

G_DEFINE_TYPE(MyApplication, my_application, GTK_TYPE_APPLICATION)

// Called when first Flutter frame received.
static void first_frame_cb(MyApplication* self, FlView* view) {
  gtk_widget_show(gtk_widget_get_toplevel(GTK_WIDGET(view)));
}

// Implements GApplication::activate.
static void my_application_activate(GApplication* application) {
  MyApplication* self = MY_APPLICATION(application);
  GtkWindow* window =
      GTK_WINDOW(gtk_application_window_new(GTK_APPLICATION(application)));

  gboolean use_header_bar = TRUE;
#ifdef GDK_WINDOWING_X11
  GdkScreen* screen = gtk_window_get_screen(window);
  if (GDK_IS_X11_SCREEN(screen)) {
    const gchar* wm_name = gdk_x11_screen_get_window_manager_name(screen);
    if (g_strcmp0(wm_name, "GNOME Shell") != 0) {
      use_header_bar = FALSE;
    }
  }
#endif
  if (use_header_bar) {
    GtkHeaderBar* header_bar = GTK_HEADER_BAR(gtk_header_bar_new());
    gtk_widget_show(GTK_WIDGET(header_bar));
    gtk_header_bar_set_title(header_bar, "DavDeck");
    gtk_header_bar_set_show_close_button(header_bar, TRUE);
    gtk_window_set_titlebar(window, GTK_WIDGET(header_bar));
  } else {
    gtk_window_set_title(window, "DavDeck");
  }

  gtk_window_set_default_size(window, 1280, 720);
  g_autoptr(FlDartProject) project = fl_dart_project_new();
  fl_dart_project_set_dart_entrypoint_arguments(
      project, self->dart_entrypoint_arguments);

  FlView* view = fl_view_new(project);
  GdkRGBA background_color;
  gdk_rgba_parse(&background_color, "#000000");
  fl_view_set_background_color(view, &background_color);
  gtk_widget_show(GTK_WIDGET(view));
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));
  g_signal_connect_swapped(view, "first-frame", G_CALLBACK(first_frame_cb),
                           self);
  gtk_widget_realize(GTK_WIDGET(view));
  fl_register_plugins(FL_PLUGIN_REGISTRY(view));
  gtk_widget_grab_focus(GTK_WIDGET(view));
}

// Implements GApplication::local_command_line.
static gboolean my_application_local_command_line(GApplication* application,
                                                  gchar*** arguments,
                                                  int* exit_status) {
  MyApplication* self = MY_APPLICATION(application);
  self->dart_entrypoint_arguments = g_strdupv(*arguments + 1);

  g_autoptr(GError) error = nullptr;
  if (!g_application_register(application, nullptr, &error)) {
    g_warning("Failed to register: %s", error->message);
    *exit_status = 1;
    return TRUE;
  }
  g_application_activate(application);
  *exit_status = 0;
  return TRUE;
}

static void my_application_startup(GApplication* application) {
  G_APPLICATION_CLASS(my_application_parent_class)->startup(application);
  StartBundledDaemonIfNeeded();
}

static void my_application_shutdown(GApplication* application) {
  StopBundledDaemonIfOwned();
  G_APPLICATION_CLASS(my_application_parent_class)->shutdown(application);
}

static void my_application_dispose(GObject* object) {
  MyApplication* self = MY_APPLICATION(object);
  g_clear_pointer(&self->dart_entrypoint_arguments, g_strfreev);
  G_OBJECT_CLASS(my_application_parent_class)->dispose(object);
}

static void my_application_class_init(MyApplicationClass* klass) {
  G_APPLICATION_CLASS(klass)->activate = my_application_activate;
  G_APPLICATION_CLASS(klass)->local_command_line =
      my_application_local_command_line;
  G_APPLICATION_CLASS(klass)->startup = my_application_startup;
  G_APPLICATION_CLASS(klass)->shutdown = my_application_shutdown;
  G_OBJECT_CLASS(klass)->dispose = my_application_dispose;
}

static void my_application_init(MyApplication* self) {}

MyApplication* my_application_new() {
  g_set_prgname(APPLICATION_ID);
  return MY_APPLICATION(g_object_new(my_application_get_type(),
                                     "application-id", APPLICATION_ID, "flags",
                                     G_APPLICATION_NON_UNIQUE, nullptr));
}

#include <flutter/dart_project.h>
#include <flutter/flutter_view_controller.h>

#include <filesystem>
#include <cwctype>
#include <cstdlib>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>
#include <utility>
#include <vector>

#include <windows.h>
#include <winhttp.h>

#pragma comment(lib, "winhttp.lib")

#include "flutter_window.h"
#include "utils.h"

namespace {

struct BundledRuntime {
  std::filesystem::path daemon;
  std::filesystem::path caddy;
};

struct OwnedDaemon {
  HANDLE process = nullptr;
  HANDLE job = nullptr;
};

std::optional<OwnedDaemon> owned_daemon;

bool IsRegularFile(const std::filesystem::path &path) {
  std::error_code error;
  return std::filesystem::is_regular_file(path, error);
}

std::filesystem::path CurrentExecutablePath() {
  std::vector<wchar_t> buffer(MAX_PATH);
  for (;;) {
    const DWORD length = GetModuleFileNameW(
        nullptr, buffer.data(), static_cast<DWORD>(buffer.size()));
    if (length == 0) {
      return {};
    }
    if (length < buffer.size() - 1) {
      return std::filesystem::path(std::wstring(buffer.data(), length));
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
  for (int level = 0; level < 6 && !directory.empty(); ++level) {
    const auto daemon = directory / L"bin" / L"davd.exe";
    const auto caddy = directory / L"libexec" / L"caddy.exe";
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

std::wstring QuoteCommandLinePath(const std::filesystem::path &path) {
  return L"\"" + path.wstring() + L"\"";
}

void LogCreateProcessFailure(DWORD error) {
  OutputDebugStringW((std::wstring(L"DavDeck: could not start bundled daemon; "
                                   L"CreateProcessW error ") +
                      std::to_wstring(error) + L"\n")
                         .c_str());
}

std::optional<std::wstring> ReadUtf8File(const std::filesystem::path &path) {
  std::ifstream file(path, std::ios::binary);
  if (!file) {
    return std::nullopt;
  }
  const std::string content((std::istreambuf_iterator<char>(file)),
                            std::istreambuf_iterator<char>());
  if (content.empty()) {
    return std::nullopt;
  }
  const int length = MultiByteToWideChar(CP_UTF8, 0, content.data(),
                                         static_cast<int>(content.size()),
                                         nullptr, 0);
  if (length <= 0) {
    return std::nullopt;
  }
  std::wstring result(length, L'\0');
  if (MultiByteToWideChar(CP_UTF8, 0, content.data(),
                          static_cast<int>(content.size()), result.data(),
                          length) != length) {
    return std::nullopt;
  }
  while (!result.empty() && iswspace(result.back())) {
    result.pop_back();
  }
  return result.empty() ? std::nullopt
                        : std::optional<std::wstring>(std::move(result));
}

struct DaemonConnection {
  std::wstring endpoint;
  std::wstring token;
};

std::optional<std::wstring> ReadEnvironmentVariable(const wchar_t *name) {
  wchar_t *value = nullptr;
  size_t value_size = 0;
  if (_wdupenv_s(&value, &value_size, name) != 0 || value == nullptr) {
    std::free(value);
    return std::nullopt;
  }
  std::wstring result(value);
  std::free(value);
  return result.empty() ? std::nullopt
                        : std::optional<std::wstring>(std::move(result));
}

std::optional<DaemonConnection> ReadDaemonConnection() {
  const auto local_app_data = ReadEnvironmentVariable(L"LOCALAPPDATA");
  const auto app_data = ReadEnvironmentVariable(L"APPDATA");
  if (!local_app_data.has_value() || !app_data.has_value()) {
    return std::nullopt;
  }
  const auto endpoint_path = std::filesystem::path(*local_app_data) /
                             L"DavDeck" / L"run" / L"management.endpoint";
  const auto token_path = std::filesystem::path(*app_data) / L"DavDeck" /
                          L"management.token";
  auto endpoint = ReadUtf8File(endpoint_path);
  auto token = ReadUtf8File(token_path);
  if (!endpoint.has_value() || !token.has_value()) {
    return std::nullopt;
  }
  return DaemonConnection{std::move(*endpoint), std::move(*token)};
}

bool RequestDaemon(const wchar_t *method, const wchar_t *path,
                   DWORD expected_status) {
  const auto connection = ReadDaemonConnection();
  if (!connection.has_value()) {
    return false;
  }

  URL_COMPONENTS components{};
  components.dwStructSize = sizeof(components);
  components.dwSchemeLength = static_cast<DWORD>(-1);
  components.dwHostNameLength = static_cast<DWORD>(-1);
  components.dwUrlPathLength = static_cast<DWORD>(-1);
  if (!WinHttpCrackUrl(connection->endpoint.c_str(), 0, 0, &components)) {
    return false;
  }
  const std::wstring host(components.lpszHostName, components.dwHostNameLength);
  const bool secure = components.nScheme == INTERNET_SCHEME_HTTPS;
  HINTERNET session = WinHttpOpen(L"DavDeck GUI", WINHTTP_ACCESS_TYPE_NO_PROXY,
                                   WINHTTP_NO_PROXY_NAME,
                                   WINHTTP_NO_PROXY_BYPASS, 0);
  if (session == nullptr) {
    return false;
  }
  HINTERNET connect = WinHttpConnect(session, host.c_str(), components.nPort, 0);
  if (connect == nullptr) {
    WinHttpCloseHandle(session);
    return false;
  }
  HINTERNET request = WinHttpOpenRequest(
      connect, method, path, nullptr,
      WINHTTP_NO_REFERER, WINHTTP_DEFAULT_ACCEPT_TYPES,
      secure ? WINHTTP_FLAG_SECURE : 0);
  if (request == nullptr) {
    WinHttpCloseHandle(connect);
    WinHttpCloseHandle(session);
    return false;
  }
  const std::wstring authorization = L"Authorization: Bearer " + connection->token;
  const bool sent = WinHttpAddRequestHeaders(
                        request, authorization.c_str(), static_cast<DWORD>(-1),
                        WINHTTP_ADDREQ_FLAG_ADD | WINHTTP_ADDREQ_FLAG_REPLACE) &&
                    WinHttpSendRequest(request, WINHTTP_NO_ADDITIONAL_HEADERS,
                                       0, WINHTTP_NO_REQUEST_DATA, 0, 0, 0) &&
                    WinHttpReceiveResponse(request, nullptr);
  DWORD status = 0;
  DWORD status_size = sizeof(status);
  if (sent) {
    WinHttpQueryHeaders(request,
                        WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                        WINHTTP_HEADER_NAME_BY_INDEX, &status, &status_size,
                        WINHTTP_NO_HEADER_INDEX);
  }
  WinHttpCloseHandle(request);
  WinHttpCloseHandle(connect);
  WinHttpCloseHandle(session);
  return sent && status == expected_status;
}

bool DaemonIsReachable() {
  return RequestDaemon(L"GET", L"/api/v1/status", HTTP_STATUS_OK);
}

bool RequestDaemonShutdown() {
  return RequestDaemon(L"POST", L"/api/v1/daemon/shutdown", HTTP_STATUS_OK);
}

HANDLE CreateDaemonJob() {
  HANDLE job = CreateJobObjectW(nullptr, nullptr);
  if (job == nullptr) {
    return nullptr;
  }
  JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};
  limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
  if (!SetInformationJobObject(job, JobObjectExtendedLimitInformation, &limits,
                               sizeof(limits))) {
    CloseHandle(job);
    return nullptr;
  }
  return job;
}

void StartBundledDaemonIfPresent() {
  const auto runtime = FindBundledRuntime();
  if (!runtime.has_value()) {
    // Development builds and headless packages do not have a bundled GUI
    // runtime. In those cases the GUI continues to discover an externally
    // started daemon through the normal management endpoint.
    return;
  }
  if (DaemonIsReachable()) {
    return;
  }

  std::wstring command_line = QuoteCommandLinePath(runtime->daemon);
  command_line += L" --caddy-binary ";
  command_line += QuoteCommandLinePath(runtime->caddy);
  command_line += L" --portable-owner gui";
  std::vector<wchar_t> mutable_command_line(command_line.begin(),
                                             command_line.end());
  mutable_command_line.push_back(L'\0');

  STARTUPINFOW startup_info{};
  startup_info.cb = sizeof(startup_info);
  PROCESS_INFORMATION process_info{};
  if (!CreateProcessW(nullptr, mutable_command_line.data(), nullptr, nullptr,
                      FALSE, CREATE_NO_WINDOW, nullptr, nullptr,
                      &startup_info, &process_info)) {
    LogCreateProcessFailure(GetLastError());
    return;
  }

  CloseHandle(process_info.hThread);
  HANDLE job = CreateDaemonJob();
  if (job != nullptr && !AssignProcessToJobObject(job, process_info.hProcess)) {
    CloseHandle(job);
    job = nullptr;
  }
  if (WaitForSingleObject(process_info.hProcess, 200) == WAIT_OBJECT_0) {
    if (job != nullptr) {
      CloseHandle(job);
    }
    CloseHandle(process_info.hProcess);
    return;
  }
  owned_daemon = OwnedDaemon{process_info.hProcess, job};
}

void StopBundledDaemonIfOwned() {
  if (!owned_daemon.has_value()) {
    return;
  }
  if (WaitForSingleObject(owned_daemon->process, 0) == WAIT_TIMEOUT) {
    RequestDaemonShutdown();
    if (WaitForSingleObject(owned_daemon->process, 5000) == WAIT_TIMEOUT) {
      if (owned_daemon->job != nullptr) {
        CloseHandle(owned_daemon->job);
        owned_daemon->job = nullptr;
      } else {
        TerminateProcess(owned_daemon->process, 1);
      }
    }
  }
  CloseHandle(owned_daemon->process);
  if (owned_daemon->job != nullptr) {
    CloseHandle(owned_daemon->job);
  }
  owned_daemon.reset();
}

}  // namespace

int APIENTRY wWinMain(_In_ HINSTANCE instance, _In_opt_ HINSTANCE prev,
                      _In_ wchar_t *command_line, _In_ int show_command) {
  // Attach to console when present (e.g., 'flutter run') or create a
  // new console when running with a debugger.
  if (!::AttachConsole(ATTACH_PARENT_PROCESS) && ::IsDebuggerPresent()) {
    CreateAndAttachConsole();
  }

  // Initialize COM, so that it is available for use in the library and/or
  // plugins.
  ::CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);

  StartBundledDaemonIfPresent();

  flutter::DartProject project(L"data");

  std::vector<std::string> command_line_arguments =
      GetCommandLineArguments();

  project.set_dart_entrypoint_arguments(std::move(command_line_arguments));

  FlutterWindow window(project);
  Win32Window::Point origin(10, 10);
  Win32Window::Size size(1280, 720);
  if (!window.Create(L"DavDeck", origin, size)) {
    StopBundledDaemonIfOwned();
    return EXIT_FAILURE;
  }
  window.SetQuitOnClose(true);

  ::MSG msg;
  while (::GetMessage(&msg, nullptr, 0, 0)) {
    ::TranslateMessage(&msg);
    ::DispatchMessage(&msg);
  }

  StopBundledDaemonIfOwned();

  ::CoUninitialize();
  return EXIT_SUCCESS;
}

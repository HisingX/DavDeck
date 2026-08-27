import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

const davDeckLogoAsset = 'assets/branding/davdeck_logo.png';
const davDeckProjectUrl = 'https://github.com/HisingX/DavDeck';

class AboutPage extends StatelessWidget {
  const AboutPage({super.key, required this.controller});

  final StatusController controller;

  Future<void> _copyProjectAddress(BuildContext context) async {
    final strings = AppStrings.of(context);
    await Clipboard.setData(const ClipboardData(text: davDeckProjectUrl));
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(strings.projectAddressCopied)));
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final version = controller.status?.version;
        final versionLabel = version == null || version.isEmpty
            ? strings.versionLoading
            : '${strings.version} $version';
        return SingleChildScrollView(
          padding: appPagePadding(context),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 980),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  AppPageHeader(
                    title: strings.about,
                    subtitle: strings.aboutSubtitle,
                  ),
                  const SizedBox(height: 28),
                  AppSurface(
                    padding: const EdgeInsets.all(28),
                    child: LayoutBuilder(
                      builder: (context, constraints) {
                        final brand = Row(
                          crossAxisAlignment: CrossAxisAlignment.center,
                          children: [
                            Semantics(
                              label: strings.logoDescription,
                              image: true,
                              child: Image.asset(
                                davDeckLogoAsset,
                                width: 116,
                                height: 116,
                                fit: BoxFit.contain,
                              ),
                            ),
                            const SizedBox(width: 24),
                            Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  'DavDeck',
                                  style: theme.textTheme.headlineMedium
                                      ?.copyWith(
                                        color: scheme.onSurface,
                                        fontWeight: FontWeight.w800,
                                        letterSpacing: -0.7,
                                      ),
                                ),
                                const SizedBox(height: 6),
                                Text(
                                  versionLabel,
                                  style: theme.textTheme.bodyMedium?.copyWith(
                                    color: scheme.onSurfaceVariant,
                                    fontWeight: FontWeight.w600,
                                  ),
                                ),
                              ],
                            ),
                          ],
                        );

                        final description = Text(
                          strings.aboutProjectDescription,
                          style: theme.textTheme.bodyLarge?.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        );
                        if (constraints.maxWidth < 620) {
                          return Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              brand,
                              const SizedBox(height: 24),
                              description,
                            ],
                          );
                        }
                        return Column(
                          children: [
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.center,
                              children: [
                                brand,
                                const SizedBox(width: 24),
                                Expanded(child: description),
                              ],
                            ),
                          ],
                        );
                      },
                    ),
                  ),
                  const SizedBox(height: 20),
                  AppSurface(
                    padding: const EdgeInsets.fromLTRB(24, 22, 16, 22),
                    child: Row(
                      children: [
                        Icon(Icons.link, color: scheme.primary),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                strings.projectAddress,
                                style: theme.textTheme.titleMedium?.copyWith(
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                              const SizedBox(height: 4),
                              SelectableText(
                                davDeckProjectUrl,
                                style: theme.textTheme.bodyMedium?.copyWith(
                                  color: scheme.primary,
                                ),
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                          tooltip: strings.copyProjectAddress,
                          onPressed: () => _copyProjectAddress(context),
                          icon: const Icon(Icons.content_copy_outlined),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 20),
                  LayoutBuilder(
                    builder: (context, constraints) {
                      final cards = [
                        _AboutInfoCard(
                          icon: Icons.code,
                          title: strings.aboutOpenSource,
                          body: strings.aboutOpenSourceDescription,
                        ),
                        _AboutInfoCard(
                          icon: Icons.gavel_outlined,
                          title: strings.aboutLicense,
                          body: strings.aboutLicenseDescription,
                        ),
                      ];
                      if (constraints.maxWidth < 700) {
                        return Column(
                          children: [
                            for (final card in cards) ...[
                              card,
                              if (card != cards.last)
                                const SizedBox(height: 16),
                            ],
                          ],
                        );
                      }
                      return Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          for (var i = 0; i < cards.length; i++) ...[
                            Expanded(child: cards[i]),
                            if (i != cards.length - 1)
                              const SizedBox(width: 16),
                          ],
                        ],
                      );
                    },
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

class _AboutInfoCard extends StatelessWidget {
  const _AboutInfoCard({
    required this.icon,
    required this.title,
    required this.body,
  });

  final IconData icon;
  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return AppSurface(
      padding: const EdgeInsets.all(22),
      shadow: false,
      color: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: scheme.primary, size: 28),
          const SizedBox(height: 18),
          Text(
            title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            body,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

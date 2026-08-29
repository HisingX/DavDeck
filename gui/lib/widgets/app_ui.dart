import 'package:flutter/material.dart';

/// Shows an application dialog that must be closed through an explicit action.
///
/// Dialogs in DavDeck often contain form state or information that users may
/// need to copy. Keeping the modal open when the barrier is tapped prevents an
/// accidental click outside the dialog from discarding that context.
Future<T?> showAppDialog<T>({
  required BuildContext context,
  required WidgetBuilder builder,
}) => showDialog<T>(
  context: context,
  barrierDismissible: false,
  builder: builder,
);

double appPageInset(BuildContext context) =>
    MediaQuery.sizeOf(context).width < 900 ? 20 : 40;

EdgeInsets appPagePadding(BuildContext context) {
  final inset = appPageInset(context);
  final top = MediaQuery.sizeOf(context).height < 700 ? 20.0 : inset;
  return EdgeInsets.fromLTRB(inset, top, inset, 48);
}

class AppPageHeader extends StatelessWidget {
  const AppPageHeader({
    super.key,
    required this.title,
    required this.subtitle,
    this.actions,
  });

  final String title;
  final String subtitle;
  final Widget? actions;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return LayoutBuilder(
      builder: (context, constraints) {
        final titleBlock = Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: theme.textTheme.headlineMedium?.copyWith(
                fontWeight: FontWeight.w700,
                letterSpacing: -0.5,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              subtitle,
              style: theme.textTheme.bodyLarge?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        );
        if (actions == null) return titleBlock;

        if (constraints.maxWidth < 760) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              titleBlock,
              const SizedBox(height: 18),
              Align(alignment: Alignment.centerRight, child: actions),
            ],
          );
        }

        return Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(child: titleBlock),
            const SizedBox(width: 24),
            actions!,
          ],
        );
      },
    );
  }
}

class AppSurface extends StatelessWidget {
  const AppSurface({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(20),
    this.color,
    this.borderRadius = 14,
    this.shadow = true,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;
  final Color? color;
  final double borderRadius;
  final bool shadow;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: color ?? theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(borderRadius),
        border: Border.all(color: theme.colorScheme.outlineVariant),
        boxShadow: shadow
            ? [
                BoxShadow(
                  color: theme.colorScheme.shadow.withValues(alpha: 0.055),
                  blurRadius: 18,
                  offset: const Offset(0, 6),
                ),
              ]
            : null,
      ),
      child: child,
    );
  }
}

class AppStatusPill extends StatelessWidget {
  const AppStatusPill({
    super.key,
    required this.label,
    required this.color,
    this.icon,
  });

  final String label;
  final Color color;
  final IconData? icon;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
    decoration: BoxDecoration(
      color: color.withValues(alpha: 0.11),
      borderRadius: BorderRadius.circular(99),
    ),
    child: FittedBox(
      fit: BoxFit.scaleDown,
      alignment: Alignment.centerLeft,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 15, color: color),
            const SizedBox(width: 5),
          ],
          Text(
            label,
            style: TextStyle(
              color: color,
              fontSize: 12,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    ),
  );
}

class AppNotice extends StatelessWidget {
  const AppNotice({
    super.key,
    required this.icon,
    required this.text,
    this.color,
    this.textColor,
  });

  final IconData icon;
  final String text;
  final Color? color;
  final Color? textColor;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
      decoration: BoxDecoration(
        color: color ?? scheme.secondaryContainer,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: (textColor ?? scheme.onSecondaryContainer).withValues(
            alpha: 0.14,
          ),
        ),
      ),
      child: Row(
        children: [
          Icon(icon, color: textColor ?? scheme.onSecondaryContainer),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              text,
              style: TextStyle(color: textColor ?? scheme.onSecondaryContainer),
            ),
          ),
        ],
      ),
    );
  }
}

class AppSearchField extends StatefulWidget {
  const AppSearchField({
    super.key,
    required this.controller,
    required this.hintText,
    required this.clearTooltip,
    required this.onChanged,
    this.width,
  });

  final TextEditingController controller;
  final String hintText;
  final String clearTooltip;
  final ValueChanged<String> onChanged;
  final double? width;

  @override
  State<AppSearchField> createState() => _AppSearchFieldState();
}

class _AppSearchFieldState extends State<AppSearchField> {
  late final FocusNode _focusNode;

  @override
  void initState() {
    super.initState();
    _focusNode = FocusNode()..addListener(_handleFocusChange);
  }

  @override
  void dispose() {
    _focusNode
      ..removeListener(_handleFocusChange)
      ..dispose();
    super.dispose();
  }

  void _handleFocusChange() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return SizedBox(
      width: widget.width,
      child: Container(
        constraints: const BoxConstraints(minHeight: 54),
        padding: const EdgeInsets.only(left: 15, right: 5),
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: BorderRadius.circular(13),
          border: Border.all(
            color: _focusNode.hasFocus ? scheme.primary : scheme.outline,
            width: _focusNode.hasFocus ? 2 : 1,
          ),
        ),
        child: Row(
          children: [
            Icon(Icons.search, color: scheme.onSurfaceVariant),
            const SizedBox(width: 10),
            Expanded(
              child: Stack(
                alignment: Alignment.centerLeft,
                children: [
                  if (widget.controller.text.isEmpty)
                    IgnorePointer(
                      child: Text(
                        widget.hintText,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ),
                  EditableText(
                    controller: widget.controller,
                    focusNode: _focusNode,
                    style: theme.textTheme.bodyMedium!,
                    cursorColor: scheme.primary,
                    backgroundCursorColor: scheme.onSurface,
                    selectionColor: scheme.primary.withValues(alpha: 0.2),
                    maxLines: 1,
                    textInputAction: TextInputAction.search,
                    onChanged: (value) {
                      setState(() {});
                      widget.onChanged(value);
                    },
                    selectionControls: materialTextSelectionControls,
                  ),
                ],
              ),
            ),
            if (widget.controller.text.isNotEmpty)
              IconButton(
                tooltip: widget.clearTooltip,
                onPressed: () {
                  widget.controller.clear();
                  setState(() {});
                  widget.onChanged('');
                },
                icon: const Icon(Icons.close),
              ),
          ],
        ),
      ),
    );
  }
}

Color appStatusColor(BuildContext context, String status) => switch (status
    .toUpperCase()) {
  'PASS' ||
  'RUNNING' ||
  'READY' ||
  'ENABLED' ||
  'APPLIED' ||
  'INSTALLED' => const Color(0xff21865d),
  'WARN' || 'STARTING' || 'STOPPING' || 'PENDING' => const Color(0xffb87800),
  'ERROR' || 'FAILED' || 'NOT_INSTALLED' => Theme.of(context).colorScheme.error,
  _ => Theme.of(context).colorScheme.onSurfaceVariant,
};

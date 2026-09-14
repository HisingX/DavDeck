import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/permissions/permission_models.dart';
import 'package:flutter/material.dart';

class PermissionDropdown extends StatelessWidget {
  const PermissionDropdown({
    super.key,
    required this.value,
    required this.strings,
    required this.onChanged,
  });

  final String value;
  final AppStrings strings;
  final ValueChanged<String?>? onChanged;

  @override
  Widget build(BuildContext context) => InputDecorator(
    decoration: const InputDecoration(
      contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
    ),
    child: DropdownButtonHideUnderline(
      child: DropdownButton<String>(
        value: value,
        isExpanded: true,
        items:
            [
                  ('NONE', strings.noAccess),
                  ('READ', strings.readOnly),
                  ('READ_WRITE', strings.readWrite),
                ]
                .map(
                  (item) => DropdownMenuItem<String>(
                    value: item.$1,
                    child: Text(item.$2, overflow: TextOverflow.ellipsis),
                  ),
                )
                .toList(growable: false),
        onChanged: onChanged,
      ),
    ),
  );
}

class PermissionSaveIndicator extends StatelessWidget {
  const PermissionSaveIndicator({
    super.key,
    required this.state,
    required this.strings,
  });

  final PermissionSaveState state;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return switch (state) {
      PermissionSaveState.idle => Text(
        strings.notModified,
        style: TextStyle(color: scheme.onSurfaceVariant),
      ),
      PermissionSaveState.saving => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 15,
            height: 15,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(width: 8),
          Text(strings.saving),
        ],
      ),
      PermissionSaveState.saved => _status(
        context,
        Icons.check_circle_outline,
        strings.saved,
        scheme.primary,
      ),
      PermissionSaveState.error => _status(
        context,
        Icons.warning_amber_rounded,
        strings.saveFailed,
        scheme.error,
      ),
    };
  }

  Widget _status(
    BuildContext context,
    IconData icon,
    String text,
    Color color,
  ) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Icon(icon, size: 17, color: color),
      const SizedBox(width: 7),
      Text(text, style: TextStyle(color: color)),
    ],
  );
}

enum PermissionFilter { all, authorized }

class PermissionFilterBar extends StatelessWidget {
  const PermissionFilterBar({
    super.key,
    required this.searchController,
    required this.searchHint,
    required this.allLabel,
    required this.filter,
    required this.strings,
    required this.onFilterChanged,
    required this.onSearchChanged,
  });

  final TextEditingController searchController;
  final String searchHint;
  final String allLabel;
  final PermissionFilter filter;
  final AppStrings strings;
  final ValueChanged<PermissionFilter> onFilterChanged;
  final ValueChanged<String> onSearchChanged;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final segmented = SegmentedButton<PermissionFilter>(
        segments: [
          ButtonSegment(value: PermissionFilter.all, label: Text(allLabel)),
          ButtonSegment(
            value: PermissionFilter.authorized,
            label: Text(strings.authorized),
          ),
        ],
        selected: {filter},
        onSelectionChanged: (selection) => onFilterChanged(selection.first),
        showSelectedIcon: false,
      );
      final search = TextField(
        controller: searchController,
        onChanged: onSearchChanged,
        decoration: InputDecoration(
          prefixIcon: const Icon(Icons.search),
          hintText: searchHint,
          suffixIcon: searchController.text.isEmpty
              ? null
              : IconButton(
                  tooltip: strings.clearSearch,
                  onPressed: () {
                    searchController.clear();
                    onSearchChanged('');
                  },
                  icon: const Icon(Icons.close),
                ),
        ),
      );
      if (constraints.maxWidth < 610) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [search, const SizedBox(height: 10), segmented],
        );
      }
      return Row(
        children: [
          Expanded(child: search),
          const SizedBox(width: 12),
          segmented,
        ],
      );
    },
  );
}

String permissionLabel(AppStrings strings, String permission) =>
    switch (permission) {
      'READ' => strings.readOnly,
      'READ_WRITE' => strings.readWrite,
      _ => strings.noAccess,
    };

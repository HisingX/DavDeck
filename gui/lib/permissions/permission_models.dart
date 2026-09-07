enum PermissionSaveState { idle, saving, saved, error }

String permissionKey(String firstId, String secondId) => '$firstId:$secondId';

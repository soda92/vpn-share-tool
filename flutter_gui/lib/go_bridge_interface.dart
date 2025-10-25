abstract class GoBridge {
  void start();
  void shareUrl(String url);
  void pollEvents(dynamic sendPort);
  void startForegroundService();
  void stopForegroundService();
  Future<String?> getIP();
  void startGoBackend();
  Future<bool> hasNotificationPermission();
  Future<bool> isForegroundServiceRunning();
}

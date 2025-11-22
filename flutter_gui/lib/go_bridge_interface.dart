abstract class GoBridge {
  void shareUrl(String url);
  void pollEvents(dynamic sendPort);
  void startForegroundService();
  void stopForegroundService();
  Future<String?> getIP();
  void startGoBackendWithPort(int port);
  Future<bool> hasNotificationPermission();
  Future<bool> isForegroundServiceRunning();
  void setDeviceIP(String ip);
}

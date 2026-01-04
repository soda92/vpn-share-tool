import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:network_info_plus/network_info_plus.dart'; // Add this import
import 'package:path_provider/path_provider.dart'; // Add this import

import 'package:vpn_share_tool/go_bridge_interface.dart';
import 'package:vpn_share_tool/go_bridge.dart'; // Import Android bridge
import 'package:vpn_share_tool/go_bridge_ffi.dart'; // Import Linux FFI bridge

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'VPN Share Tool',
      theme: ThemeData(primarySwatch: Colors.blue, useMaterial3: true),
      home: const MyHomePage(),
    );
  }
}

class MyHomePage extends StatefulWidget {
  const MyHomePage({super.key});

  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

class _MyHomePageState extends State<MyHomePage> {
  String _ipAddress = 'Loading...';
  final List<Map<String, dynamic>> _proxies = [];
  final _urlController = TextEditingController();
  late final GoBridge _bridge;
  int _apiPort = 0; // Store the found API port

  bool _isForegroundServiceActive = false;
  bool _hasNotificationPermission = false;

  @override
  void initState() {
    super.initState();
    _initializeGoBackend();
    _initializeServiceState();
  }

  Future<void> _initializeGoBackend() async {
    if (Platform.isLinux) {
      print("Instantiating Linux FFI GoBridge");
      _bridge = GoBridgeLinux();
    } else {
      print("Instantiating Android GoBridge");
      _bridge = GoBridgeAndroid();
    }
    
    // Detect and set IP using network_info_plus
    try {
      final info = NetworkInfo();
      var wifiIP = await info.getWifiIP();
      if (wifiIP != null && wifiIP.isNotEmpty) {
        print("Detected WiFi IP: $wifiIP");
        _bridge.setDeviceIP(wifiIP);
      } else {
        print("Could not detect WiFi IP or not connected to WiFi.");
      }
    } catch (e) {
      print("Error detecting WiFi IP: $e");
    }
    
    // Set storage path for debug database
    try {
       final directory = await getApplicationDocumentsDirectory();
       print("Setting storage path to: ${directory.path}");
       _bridge.setStoragePath(directory.path);
    } catch (e) {
       print("Error setting storage path: $e");
    }

    _apiPort = await _findAvailablePort(10081); // Start searching from 10081
    print("Found available port: $_apiPort");

    _startListeningEvents(); // Listen to stream for both platforms
    _bridge.startGoBackendWithPort(
      _apiPort,
    ); // Start the core Go backend with the found portX
  }

  Future<int> _findAvailablePort(int startPort) async {
    for (int port = startPort; port < startPort + 100; port++) {
      try {
        final socket = await ServerSocket.bind(InternetAddress.anyIPv4, port);
        await socket.close();
        return port;
      } catch (e) {
        // Port is not available, try next one
      }
    }
    throw Exception(
      "No available port found in range $startPort-${startPort + 99}",
    );
  }

  Future<void> _initializeServiceState() async {
    if (Platform.isAndroid) {
      await _checkServiceStatus();
    }
  }

  Future<void> _checkServiceStatus() async {
    if (Platform.isAndroid) {
      final androidBridge = _bridge as GoBridgeAndroid;
      _hasNotificationPermission = await androidBridge
          .hasNotificationPermission();
      _isForegroundServiceActive = await androidBridge
          .isForegroundServiceRunning();

      if (_hasNotificationPermission && !_isForegroundServiceActive) {
        // Start silently if permission is already granted and not running
        androidBridge.startForegroundService();
        _isForegroundServiceActive =
            true; // Optimistically assume it starts successfully
      }
      setState(() {}); // Update UI based on initial state
    }
  }

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  void _startListeningEvents() {
    if (Platform.isLinux) {
      (_bridge as GoBridgeLinux).eventStream.listen((event) {
        if (mounted) {
          setState(() {
            if (event['type'] == 'ip_ready') {
              _ipAddress = event['ip'];
            } else if (event['type'] == 'added') {
              _proxies.removeWhere(
                (p) => p['original_url'] == event['proxy']['original_url'],
              );
              _proxies.add(event['proxy']);
            } else if (event['type'] == 'removed') {
              _proxies.removeWhere(
                (p) => p['original_url'] == event['proxy']['original_url'],
              );
            } else if (event['type'] == 'error') {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('Error from Go: ${event['message']}')),
              );
            }
          });
        }
      });
    } else {
      (_bridge as GoBridgeAndroid).eventStream.listen((event) {
        if (mounted) {
          setState(() {
            if (event['type'] == 'ip_ready') {
              _ipAddress = event['ip'];
            } else if (event['type'] == 'added') {
              _proxies.removeWhere(
                (p) => p['original_url'] == event['proxy']['original_url'],
              );
              _proxies.add(event['proxy']);
            } else if (event['type'] == 'removed') {
              _proxies.removeWhere(
                (p) => p['original_url'] == event['proxy']['original_url'],
              );
            } else if (event['type'] == 'error') {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('Error from Go: ${event['message']}')),
              );
            }
          });
        }
      });
    }
  }

  void _shareUrl() {
    if (_urlController.text.isNotEmpty) {
      _bridge.shareUrl(_urlController.text);
      _urlController.clear();
      FocusScope.of(context).unfocus();
    }
  }

  @override
  Widget build(BuildContext context) {
    final platform = Platform.isLinux ? "Linux (FFI)" : "Android";
    return Scaffold(
      appBar: AppBar(title: Text('VPN Share Tool ($platform)')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(
              'Your IP Address is: $_ipAddress',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _urlController,
                    decoration: const InputDecoration(
                      labelText: 'URL to share',
                      border: OutlineInputBorder(),
                    ),
                    onSubmitted: (_) => _shareUrl(),
                  ),
                ),
                const SizedBox(width: 10),
                ElevatedButton(
                  onPressed: _shareUrl,
                  child: const Text('Share'),
                ),
              ],
            ),
            const SizedBox(height: 20),
            if (Platform.isAndroid &&
                !_isForegroundServiceActive) // Only show on Android if not active
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  ElevatedButton(
                    onPressed: () async {
                      // This will trigger permission request if not granted and then start service
                      (_bridge as GoBridgeAndroid).startForegroundService();
                      // Optimistically update UI, actual state will be reflected by events later
                      setState(() {
                        _isForegroundServiceActive = true;
                      });
                    },
                    child: const Text('Enable Background Service'),
                  ),
                ],
              ),
            const SizedBox(height: 20),
            Text('Shared URLs', style: Theme.of(context).textTheme.titleLarge),
            const Divider(),
            Expanded(
              child: ListView.builder(
                itemCount: _proxies.length,
                itemBuilder: (context, index) {
                  final proxy = _proxies[index];
                  final sharedUrl =
                      'http://$_ipAddress:${proxy['remote_port']}${proxy['path']}';
                  return Card(
                    child: ListTile(
                      title: Text(proxy['original_url'] ?? 'Invalid URL'),
                      subtitle: Text(sharedUrl),
                      trailing: IconButton(
                        icon: const Icon(Icons.copy),
                        onPressed: () {
                          Clipboard.setData(ClipboardData(text: sharedUrl));
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(
                              content: Text('Copied to clipboard'),
                            ),
                          );
                        },
                      ),
                    ),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

import 'dart:convert';
import 'dart:io';
import 'dart:isolate';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

// Conditional imports for platform-specific bridge
import 'package:flutter_gui/go_bridge.dart' if (dart.library.ffi) 'package:flutter_gui/go_bridge_ffi.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'VPN Share Tool',
      theme: ThemeData(
        primarySwatch: Colors.blue,
        useMaterial3: true,
      ),
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
  Isolate? _pollingIsolate;

  @override
  void initState() {
    super.initState();
    _startPolling();
    // Use the correct start method based on the platform
    if (Platform.isLinux) {
      GoBridgeFFI.start();
    } else {
      GoBridge.start();
    }
  }

  @override
  void dispose() {
    _pollingIsolate?.kill(priority: Isolate.immediate);
    super.dispose();
  }

  void _startPolling() async {
    final receivePort = ReceivePort();
    
    // Use the correct polling method based on the platform
    if (Platform.isLinux) {
      _pollingIsolate = await Isolate.spawn(GoBridgeFFI.pollEvents, receivePort.sendPort);
    } else {
       final sendPort = receivePort.sendPort;
      _pollingIsolate = await Isolate.spawn((port) async {
        // On Android, we need to initialize the plugin registrant
        DartPluginRegistrant.ensureInitialized();
        GoBridge.pollEvents(port);
      }, sendPort);
    }

    receivePort.listen((data) {
      final event = data as Map<String, dynamic>;
      if (mounted) {
        setState(() {
          if (event['type'] == 'ip_ready') {
            _ipAddress = event['ip'];
          } else if (event['type'] == 'added') {
            _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
            _proxies.add(event['proxy']);
          } else if (event['type'] == 'removed') {
            _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
          }
        });
      }
    });
  }

  void _shareUrl() {
    if (_urlController.text.isNotEmpty) {
      // Use the correct share method based on the platform
      if (Platform.isLinux) {
        GoBridgeFFI.shareUrl(_urlController.text);
      } else {
        GoBridge.shareUrl(_urlController.text);
      }
      _urlController.clear();
      FocusScope.of(context).unfocus();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('VPN Share Tool (Linux FFI)'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text('Your IP Address is: $_ipAddress', style: Theme.of(context).textTheme.titleMedium),
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
            Text('Shared URLs', style: Theme.of(context).textTheme.titleLarge),
            const Divider(),
            Expanded(
              child: ListView.builder(
                itemCount: _proxies.length,
                itemBuilder: (context, index) {
                  final proxy = _proxies[index];
                  final sharedUrl = 'http://$_ipAddress:${proxy['remote_port']}${proxy['path']}';
                  return Card(
                    child: ListTile(
                      title: Text(proxy['original_url'] ?? 'Invalid URL'),
                      subtitle: Text(sharedUrl),
                      trailing: IconButton(
                        icon: const Icon(Icons.copy),
                        onPressed: () {
                          Clipboard.setData(ClipboardData(text: sharedUrl));
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(content: Text('Copied to clipboard')),
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
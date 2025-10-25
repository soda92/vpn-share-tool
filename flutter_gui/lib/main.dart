import 'dart:convert';
import 'dart:io';
import 'dart:isolate';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

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
  late final GoBridge _bridge;

  @override
  void initState() {
    super.initState();
    if (Platform.isLinux) {
      print("Instantiating Linux FFI GoBridge");
      _bridge = GoBridgeLinux();
    } else {
      print("Instantiating Android GoBridge");
      _bridge = GoBridgeAndroid();
    }
    _startListeningEvents(); // Listen to stream for both platforms
    if (Platform.isAndroid) {
      (_bridge as GoBridgeAndroid).startForegroundService();
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
              _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
              _proxies.add(event['proxy']);
            } else if (event['type'] == 'removed') {
              _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
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
              _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
              _proxies.add(event['proxy']);
            } else if (event['type'] == 'removed') {
              _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
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
      appBar: AppBar(
        title: Text('VPN Share Tool ($platform)'),
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
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceEvenly,
              children: [
                ElevatedButton(
                  onPressed: () {
                    if (_bridge is GoBridgeAndroid) {
                      (_bridge as GoBridgeAndroid).stopForegroundService();
                    }
                  },
                  child: const Text('Stop Service'),
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
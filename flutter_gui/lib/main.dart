import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_gui/go_bridge.dart';

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

  @override
  void initState() {
    super.initState();
    GoBridge.start();
    GoBridge.pollEvents().listen((event) {
      setState(() {
        if (event['type'] == 'ip_ready') {
          _ipAddress = event['ip'];
        } else if (event['type'] == 'added') {
          _proxies.add(event['proxy']);
        } else if (event['type'] == 'removed') {
          _proxies.removeWhere((p) => p['original_url'] == event['proxy']['original_url']);
        }
      });
    });
  }

  void _shareUrl() {
    if (_urlController.text.isNotEmpty) {
      GoBridge.shareUrl(_urlController.text);
      _urlController.clear();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('VPN Share Tool'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text('Your IP Address is: $_ipAddress', style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 20),
            TextField(
              controller: _urlController,
              decoration: const InputDecoration(
                labelText: 'URL to share',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 10),
            ElevatedButton(
              onPressed: _shareUrl,
              child: const Text('Share'),
            ),
            const SizedBox(height: 20),
            Text('Shared URLs', style: Theme.of(context).textTheme.titleLarge),
            Expanded(
              child: ListView.builder(
                itemCount: _proxies.length,
                itemBuilder: (context, index) {
                  final proxy = _proxies[index];
                  final sharedUrl = 'http://$_ipAddress:${proxy['remote_port']}${proxy['path']}';
                  return ListTile(
                    title: Text(proxy['original_url']),
                    subtitle: Text(sharedUrl),
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
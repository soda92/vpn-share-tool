import 'package:flutter/material.dart';
import 'package:network_info_plus/network_info_plus.dart';

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
  String? _ipAddress;

  @override
  void initState() {
    super.initState();
    _getIpAddress();
  }

  Future<void> _getIpAddress() async {
    final networkInfo = NetworkInfo();
    String? ipAddress;
    try {
      ipAddress = await networkInfo.getWifiIP();
    } catch (e) {
      print('Failed to get IP address: $e');
    }

    setState(() {
      _ipAddress = ipAddress;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('VPN Share Tool'),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            const Text(
              'Your IP Address is:',
            ),
            Text(
              _ipAddress ?? 'Loading...',
              style: Theme.of(context).textTheme.headlineMedium,
            ),
          ],
        ),
      ),
    );
  }
}
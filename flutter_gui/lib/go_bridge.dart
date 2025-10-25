import 'dart:async';
import 'dart:convert';
import 'dart:isolate';
import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:flutter_gui/go_bridge_interface.dart';

// Top-level function for the Android polling isolate
@pragma('vm:entry-point')
void _androidIsolateEntrypoint(List<dynamic> args) async {
  SendPort sendPort = args[0];
  RootIsolateToken rootIsolateToken = args[1];

  BackgroundIsolateBinaryMessenger.ensureInitialized(rootIsolateToken);
  const MethodChannel _channel = MethodChannel('vpn_share_tool/go_bridge');

  while (true) {
    try {
      final eventJson = await _channel.invokeMethod('pollEvents');
      if (eventJson != null && eventJson.isNotEmpty) {
        sendPort.send(jsonDecode(eventJson));
      }
    } catch (e) {
      print("Error polling events in Android isolate: $e");
    }
    await Future.delayed(const Duration(milliseconds: 100));
  }
}

class GoBridgeAndroid implements GoBridge {
  final _channel = const MethodChannel('vpn_share_tool/go_bridge');

  @override
  void start() {
    _channel.invokeMethod('start');
  }

  @override
  void shareUrl(String url) {
    _channel.invokeMethod('shareUrl', {'url': url});
  }

  @override
  void pollEvents(dynamic args) {
    // This method is now just a wrapper to spawn the top-level isolate entrypoint
    Isolate.spawn(_androidIsolateEntrypoint, args as List<dynamic>);
  }
}
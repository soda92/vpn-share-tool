import 'dart:async';
import 'dart:convert';
import 'dart:isolate';
import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:flutter_gui/go_bridge_interface.dart';

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
  void pollEvents(dynamic sendPort) async {
    DartPluginRegistrant.ensureInitialized();
    while (true) {
      try {
        final eventJson = await _channel.invokeMethod('pollEvents');
        if (eventJson != null && eventJson.isNotEmpty) {
          (sendPort as SendPort).send(jsonDecode(eventJson));
        }
      } catch (e) {
        print("Error polling events: $e");
      }
      await Future.delayed(const Duration(milliseconds: 100));
    }
  }
}
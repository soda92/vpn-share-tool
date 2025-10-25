import 'dart:async';
import 'dart:convert';
import 'package:flutter/services.dart';

class GoBridge {
  static const _channel = MethodChannel('vpn_share_tool/go_bridge');

  static Future<void> start() async {
    await _channel.invokeMethod('start');
  }

  static Future<void> shareUrl(String url) async {
    await _channel.invokeMethod('shareUrl', {'url': url});
  }

  static Future<String> getProxies() async {
    return await _channel.invokeMethod('getProxies');
  }

  static Future<String> getIP() async {
    return await _channel.invokeMethod('getIP');
  }

  static Stream<Map<String, dynamic>> pollEvents() {
    final controller = StreamController<Map<String, dynamic>>();
    Timer.periodic(const Duration(milliseconds: 100), (timer) async {
      try {
        final eventJson = await _channel.invokeMethod('pollEvents');
        if (eventJson != null && eventJson.isNotEmpty) {
          controller.add(jsonDecode(eventJson));
        }
      } catch (e) {
        // Handle error
      }
    });
    return controller.stream;
  }
}

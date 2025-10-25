import 'dart:async';
import 'dart:convert';
import 'dart:isolate';
import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

class GoBridge {
  static const _channel = MethodChannel('vpn_share_tool/go_bridge');

  // This function is designed to be run in a separate Isolate.
  static Future<void> pollEvents(SendPort sendPort) async {
    // Required for background platform channels.
    WidgetsFlutterBinding.ensureInitialized();
    DartPluginRegistrant.ensureInitialized();

    while (true) {
      try {
        final eventJson = await _channel.invokeMethod('pollEvents');
        if (eventJson != null && eventJson.isNotEmpty) {
          sendPort.send(jsonDecode(eventJson));
        }
      } catch (e) {
        // Handle error, maybe send it back to the main isolate
        print("Error polling events: $e");
      }
      // Add a small delay to prevent spamming the channel
      await Future.delayed(const Duration(milliseconds: 100));
    }
  }

  static Future<void> start() async {
    await _channel.invokeMethod('start');
  }

  static Future<void> shareUrl(String url) async {
    await _channel.invokeMethod('shareUrl', {'url': url});
  }
}

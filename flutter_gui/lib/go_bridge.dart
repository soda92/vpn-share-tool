import 'dart:async';
import 'dart:convert';
import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:flutter_gui/go_bridge_interface.dart';

class GoBridgeAndroid implements GoBridge {
  final _channel = const MethodChannel('vpn_share_tool/go_bridge');
  final _eventStreamController = StreamController<Map<String, dynamic>>.broadcast();

  GoBridgeAndroid() {
    // Set up method call handler to receive events from Go
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'onEvent') {
        _eventStreamController.add(jsonDecode(call.arguments as String));
      }
    });
  }

  @override
  void start() {
    _channel.invokeMethod('start');
    // Register the Dart callback with Go
    _channel.invokeMethod('setEventCallback');
  }

  @override
  void shareUrl(String url) {
    _channel.invokeMethod('shareUrl', {'url': url});
  }

  // This method is no longer used for polling, but we keep it to satisfy the interface.
  // The actual event stream is exposed via eventStream.
  @override
  void pollEvents(dynamic args) {
    // No-op, events are pushed via stream
  }

  Stream<Map<String, dynamic>> get eventStream => _eventStreamController.stream;
}

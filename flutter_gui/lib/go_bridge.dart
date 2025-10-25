import 'dart:async';
import 'dart:convert';
import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:flutter_gui/go_bridge_interface.dart';

class GoBridgeAndroid implements GoBridge {
  final _methodChannel = const MethodChannel('vpn_share_tool/go_bridge');
  final _eventChannel = const EventChannel('vpn_share_tool/go_bridge_events');
  final _eventStreamController = StreamController<Map<String, dynamic>>.broadcast();

  GoBridgeAndroid() {
    // Set up method call handler to receive events from Go
    _methodChannel.setMethodCallHandler((call) async {
      print('Dart MethodChannel received: ${call.method}');
      // No longer handling 'onEvent' here, it comes from EventChannel
      return null;
    });

    // Listen to the EventChannel for events from Kotlin
    _eventChannel.receiveBroadcastStream().listen((event) {
      print('Dart EventChannel received: $event');
      _eventStreamController.add(jsonDecode(event as String));
    });
  }

  @override
  void startForegroundService() {
    _methodChannel.invokeMethod('startForegroundService');
  }

  @override
  void stopForegroundService() {
    _methodChannel.invokeMethod('stopForegroundService');
  }

  @override
  void start() {
    // The actual Go backend start and event callback setting is now handled by the foreground service.
    // We just need to start the foreground service.
    startForegroundService();
  }

  @override
  void shareUrl(String url) {
    _methodChannel.invokeMethod('shareUrl', {'url': url});
  }

  // This method is no longer used for polling, but we keep it to satisfy the interface.
  // The actual event stream is exposed via eventStream.
  @override
  void pollEvents(dynamic args) {
    // No-op, events are pushed via stream
  }

  Stream<Map<String, dynamic>> get eventStream => _eventStreamController.stream;
}

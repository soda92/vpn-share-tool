import 'dart:ffi';
import 'dart:isolate';
import 'dart:convert';
import 'dart:async';
import 'package:ffi/ffi.dart';
import 'package:flutter/services.dart';
import 'package:vpn_share_tool/go_bridge_interface.dart';

// FFI function signatures
typedef _StartFunc = Void Function();
typedef _ShareURLFunc = Void Function(Pointer<Utf8>);
typedef _SetEventCallbackFunc = Void Function(Pointer<NativeFunction<EventCallbackC>>);

// Dart function signatures
typedef _Start = void Function();
typedef _ShareURL = void Function(Pointer<Utf8>);
typedef _SetEventCallback = void Function(Pointer<NativeFunction<EventCallbackC>>);

// Callback type for Go to call Dart
typedef EventCallbackC = Void Function(Pointer<Utf8>);

@pragma('vm:entry-point')
void _eventCallback(Pointer<Utf8> eventJsonC) {
  final eventJson = eventJsonC.toDartString();
  _eventPort.send(eventJson);
}

// This is the port that the main isolate listens on for events from Go.
late SendPort _eventPort;

class GoBridgeLinux implements GoBridge {
  static final DynamicLibrary _lib = DynamicLibrary.open('libcore.so');

  static final _Start _start = _lib.lookup<NativeFunction<_StartFunc>>('Start').asFunction<_Start>();
  static final _ShareURL _shareURL = _lib.lookup<NativeFunction<_ShareURLFunc>>('ShareURL').asFunction<_ShareURL>();
  static final _SetEventCallback _setEventCallback = _lib.lookup<NativeFunction<_SetEventCallbackFunc>>('SetEventCallback').asFunction<_SetEventCallback>();

  final _eventStreamController = StreamController<Map<String, dynamic>>.broadcast();

  GoBridgeLinux() {
    // Set up a receive port for events from the Go callback
    final receivePort = ReceivePort();
    _eventPort = receivePort.sendPort;
    receivePort.listen((message) {
      _eventStreamController.add(jsonDecode(message as String));
    });

    // Register the Dart callback with Go
    _setEventCallback(Pointer.fromFunction<EventCallbackC>(_eventCallback));
  }

  @override
  void start() => _start();

  @override
  void shareUrl(String url) {
    final urlC = url.toNativeUtf8();
    _shareURL(urlC);
    malloc.free(urlC);
  }

  // This method is no longer used for polling, but we keep it to satisfy the interface.
  // The actual event stream is exposed via eventStream.
  @override
  void pollEvents(dynamic args) {
    // No-op, events are pushed via stream
  }

  @override
  void startForegroundService() {
    // No-op for Linux, as foreground service is an Android-specific concept.
  }

  @override
  void stopForegroundService() {
    // No-op for Linux, as foreground service is an Android-specific concept.
  }

  Stream<Map<String, dynamic>> get eventStream => _eventStreamController.stream;
}
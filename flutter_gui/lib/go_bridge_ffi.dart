import 'dart:ffi';
import 'dart:isolate';
import 'dart:convert';
import 'package:ffi/ffi.dart';
import 'package:flutter_gui/go_bridge_interface.dart';

// FFI function signatures
typedef _StartFunc = Void Function();
typedef _ShareURLFunc = Void Function(Pointer<Utf8>);
typedef _PollEventsFunc = Pointer<Utf8> Function();

// Dart function signatures
typedef _Start = void Function();
typedef _ShareURL = void Function(Pointer<Utf8>);
typedef _PollEvents = Pointer<Utf8> Function();

class GoBridgeLinux implements GoBridge {
  static final DynamicLibrary _lib = DynamicLibrary.open('libcore.so');

  static final _Start _start = _lib.lookup<NativeFunction<_StartFunc>>('Start').asFunction<_Start>();
  static final _ShareURL _shareURL = _lib.lookup<NativeFunction<_ShareURLFunc>>('ShareURL').asFunction<_ShareURL>();
  static final _PollEvents _pollEvents = _lib.lookup<NativeFunction<_PollEventsFunc>>('PollEvents').asFunction<_PollEvents>();

  @override
  void start() => _start();

  @override
  void shareUrl(String url) {
    final urlC = url.toNativeUtf8();
    _shareURL(urlC);
    malloc.free(urlC);
  }

  @override
  void pollEvents(dynamic sendPort) {
    while (true) {
      final eventC = _pollEvents();
      if (eventC != nullptr) {
        final eventString = eventC.toDartString();
        (sendPort as SendPort).send(jsonDecode(eventString));
      }
    }
  }
}

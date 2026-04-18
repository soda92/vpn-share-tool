import 'dart:ffi';
import 'dart:isolate';
import 'dart:convert';
import 'dart:async';
import 'package:ffi/ffi.dart';
import 'package:vpn_share_tool/go_bridge_interface.dart';

// FFI function signatures
typedef _ShareURLFunc = Void Function(Pointer<Utf8>);
typedef _GetNextEventFunc = Pointer<Utf8> Function();
typedef _FreeStringFunc = Void Function(Pointer<Utf8>);
typedef _GetIPFunc = Pointer<Utf8> Function();
typedef _GetProxiesFunc = Pointer<Utf8> Function();
typedef _StartApiServerWithPortFunc = Void Function(Int32 port);
typedef _SetDeviceIPFunc = Void Function(Pointer<Utf8>);
typedef _SetStoragePathFunc = Void Function(Pointer<Utf8>);

// Dart function signatures
typedef _ShareURL = void Function(Pointer<Utf8>);
typedef _GetNextEvent = Pointer<Utf8> Function();
typedef _FreeString = void Function(Pointer<Utf8>);
typedef _GetIP = Pointer<Utf8> Function();
typedef _GetProxies = Pointer<Utf8> Function();
typedef _StartApiServerWithPort = void Function(int port);
typedef _SetDeviceIP = void Function(Pointer<Utf8>);
typedef _SetStoragePath = void Function(Pointer<Utf8>);

class GoBridgeLinux implements GoBridge {
  static final DynamicLibrary _lib = DynamicLibrary.open('libcore.so');

  static final _ShareURL _shareURL = _lib
      .lookup<NativeFunction<_ShareURLFunc>>('ShareURL')
      .asFunction<_ShareURL>();

  static final _GetNextEvent _getNextEvent = _lib
      .lookup<NativeFunction<_GetNextEventFunc>>('GetNextEvent')
      .asFunction<_GetNextEvent>();

  static final _FreeString _freeString = _lib
      .lookup<NativeFunction<_FreeStringFunc>>('FreeString')
      .asFunction<_FreeString>();

  static final _GetIP _getIP = _lib
      .lookup<NativeFunction<_GetIPFunc>>('GetIP')
      .asFunction<_GetIP>();

  static final _GetProxies _getProxies = _lib
      .lookup<NativeFunction<_GetProxiesFunc>>('GetProxies')
      .asFunction<_GetProxies>();

  static final _StartApiServerWithPort _startGoBackendWithPort = _lib
      .lookup<NativeFunction<_StartApiServerWithPortFunc>>(
        'StartGoBackendWithPort',
      )
      .asFunction<_StartApiServerWithPort>();

  static final _SetDeviceIP _setDeviceIP = _lib
      .lookup<NativeFunction<_SetDeviceIPFunc>>('SetDeviceIP')
      .asFunction<_SetDeviceIP>();

  static final _SetStoragePath _setStoragePath = _lib
      .lookup<NativeFunction<_SetStoragePathFunc>>('SetStoragePath')
      .asFunction<_SetStoragePath>();

  final _eventStreamController =
      StreamController<Map<String, dynamic>>.broadcast();

  GoBridgeLinux() {
    _startEventLoop();
  }

  // Start a separate thread (Isolate) to poll for events from Go.
  // This avoids blocking the UI thread and prevents the "outside isolate" crash.
  void _startEventLoop() async {
    final receivePort = ReceivePort();
    // We pass the path to the library so the new isolate can open it.
    await Isolate.spawn(_eventWorker, receivePort.sendPort);

    receivePort.listen((message) {
      _eventStreamController.add(jsonDecode(message as String));
    });
  }

  // This runs in a separate background thread
  static void _eventWorker(SendPort sendPort) {
    // Re-open library in the new isolate
    final lib = DynamicLibrary.open('libcore.so');
    final getEvent = lib
        .lookup<NativeFunction<_GetNextEventFunc>>('GetNextEvent')
        .asFunction<_GetNextEvent>();
    final freeStr = lib
        .lookup<NativeFunction<_FreeStringFunc>>('FreeString')
        .asFunction<_FreeString>();

    while (true) {
      final eventC = getEvent(); // This BLOCKS until Go has an event
      final event = eventC.toDartString();
      freeStr(eventC); // Free Go-allocated string
      sendPort.send(event);
    }
  }

  @override
  void shareUrl(String url) {
    final urlC = url.toNativeUtf8();
    _shareURL(urlC);
    malloc.free(urlC);
  }

  @override
  Future<String?> getIP() async {
    final ipC = _getIP();
    final ip = ipC.toDartString();
    _freeString(ipC);
    return ip.isEmpty ? null : ip;
  }

  @override
  void startForegroundService() {}

  @override
  void stopForegroundService() {}

  @override
  void startGoBackendWithPort(int port) {
    _startGoBackendWithPort(port);
  }

  @override
  Future<bool> hasNotificationPermission() => Future.value(true);

  @override
  Future<bool> isForegroundServiceRunning() => Future.value(false);

  @override
  void setDeviceIP(String ip) {
    final ipC = ip.toNativeUtf8();
    _setDeviceIP(ipC);
    malloc.free(ipC);
  }

  @override
  void setStoragePath(String path) {
    final pathC = path.toNativeUtf8();
    _setStoragePath(pathC);
    malloc.free(pathC);
  }

  @override
  Stream<Map<String, dynamic>> get eventStream => _eventStreamController.stream;
}

package com.example.flutter_gui

import android.os.Handler
import android.os.Looper
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.EventChannel
import android.content.Intent

class MainActivity: FlutterActivity() {
    private val METHOD_CHANNEL = "vpn_share_tool/go_bridge"
    private val EVENT_CHANNEL = "vpn_share_tool/go_bridge_events"
    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel

    companion object {
        var eventSink: EventChannel.EventSink? = null
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        methodChannel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, METHOD_CHANNEL)
        eventChannel = EventChannel(flutterEngine.dartExecutor.binaryMessenger, EVENT_CHANNEL)

        eventChannel.setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, sink: EventChannel.EventSink) {
                eventSink = sink
                android.util.Log.d("MainActivity", "EventChannel onListen: EventSink set.")
            }

            override fun onCancel(arguments: Any?) {
                eventSink = null
                android.util.Log.d("MainActivity", "EventChannel onCancel: EventSink cleared.")
            }
        })

        methodChannel.setMethodCallHandler {
            call, result ->
            when (call.method) {
                "startForegroundService" -> {
                    val serviceIntent = Intent(this, VpnShareService::class.java)
                    startService(serviceIntent)
                    result.success(null)
                }
                "stopForegroundService" -> {
                    val serviceIntent = Intent(this, VpnShareService::class.java)
                    stopService(serviceIntent)
                    result.success(null)
                }
                "shareUrl" -> {
                    val url = call.argument<String>("url")
                    if (url != null) {
                        mobile.Mobile.shareURL(url)
                    }
                    result.success(null)
                }
                "getProxies" -> {
                    result.success(mobile.Mobile.getProxies())
                }
                "getIP" -> {
                    result.success(mobile.Mobile.getIP())
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }
}

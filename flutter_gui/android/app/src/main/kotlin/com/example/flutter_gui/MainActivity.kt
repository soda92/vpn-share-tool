package com.example.flutter_gui

import android.os.Handler
import android.os.Looper
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.EventChannel
import mobile.Mobile
import mobile.EventCallback

class MainActivity: FlutterActivity() {
    private val METHOD_CHANNEL = "vpn_share_tool/go_bridge"
    private val EVENT_CHANNEL = "vpn_share_tool/go_bridge_events"
    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel
    private var eventSink: EventChannel.EventSink? = null

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
                "start" -> {
                    Mobile.start()
                    result.success(null)
                }
                "shareUrl" -> {
                    val url = call.argument<String>("url")
                    if (url != null) {
                        Mobile.shareURL(url)
                    }
                    result.success(null)
                }
                "getProxies" -> {
                    result.success(Mobile.getProxies())
                }
                "getIP" -> {
                    result.success(Mobile.getIP())
                }
                "setEventCallback" -> {
                    // This is the Dart callback that Go will call
                    val dartCallback = object : mobile.EventCallback {
                        override fun onEvent(eventJSON: String?) {
                            if (eventJSON != null) {
                                android.util.Log.d("MainActivity", "Received event from Go: $eventJSON")
                                if (eventSink == null) {
                                    android.util.Log.e("MainActivity", "eventSink is null when trying to send event!")
                                } else {
                                    android.util.Log.d("MainActivity", "Sending event to Dart via EventChannel.")
                                    // Ensure event is sent on the main thread
                                    Handler(Looper.getMainLooper()).post {
                                        eventSink?.success(eventJSON)
                                    }
                                }
                            }
                        }
                    }
                    Mobile.setEventCallback(dartCallback)
                    result.success(null)
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }
}

package com.example.flutter_gui

import android.os.Handler
import android.os.Looper
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.EventChannel
import android.content.Intent
import androidx.core.content.ContextCompat // Import ContextCompat

import android.os.Build
import android.content.pm.PackageManager
import androidx.core.app.ActivityCompat

class MainActivity: FlutterActivity() {
    private val METHOD_CHANNEL = "vpn_share_tool/go_bridge"
    private val EVENT_CHANNEL = "vpn_share_tool/go_bridge_events"
    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel

    companion object {
        var eventSink: EventChannel.EventSink? = null
        private const val NOTIFICATION_PERMISSION_REQUEST_CODE = 1001
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
                    android.util.Log.d("MainActivity", "Received startForegroundService call from Dart.")
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) { // Android 13+
                        if (ContextCompat.checkSelfPermission(this, android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                            ActivityCompat.requestPermissions(this, arrayOf(android.Manifest.permission.POST_NOTIFICATIONS), NOTIFICATION_PERMISSION_REQUEST_CODE)
                            // We will try to start the service anyway, but it might fail without permission
                        }
                    }
                    val serviceIntent = Intent(this, VpnShareService::class.java)
                    ContextCompat.startForegroundService(this, serviceIntent) // Use ContextCompat
                    android.util.Log.d("MainActivity", "Sent intent to start VpnShareService using ContextCompat.")
                    result.success(null)
                }
                "stopForegroundService" -> {
                    android.util.Log.d("MainActivity", "Received stopForegroundService call from Dart.")
                    val serviceIntent = Intent(this, VpnShareService::class.java)
                    stopService(serviceIntent)
                    android.util.Log.d("MainActivity", "Sent intent to stop VpnShareService.")
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

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == NOTIFICATION_PERMISSION_REQUEST_CODE) {
            if (grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
                android.util.Log.d("MainActivity", "POST_NOTIFICATIONS permission granted.")
            } else {
                android.util.Log.d("MainActivity", "POST_NOTIFICATIONS permission denied.")
            }
        }
    }
}

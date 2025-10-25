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
                "startGoBackend" -> {
                    android.util.Log.d("MainActivity", "Received startGoBackend call from Dart.")
                    // Set event callback for Go Mobile
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
                    mobile.Mobile.setEventCallback(dartCallback)

                    // Initialize Go backend here
                    android.util.Log.d("MainActivity", "Starting Go Mobile backend...")
                    mobile.Mobile.start()
                    result.success(null)
                }
                "startForegroundService" -> {
                    android.util.Log.d("MainActivity", "Received startForegroundService call from Dart.")
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) { // Android 13+
                        if (ContextCompat.checkSelfPermission(this, android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                            android.util.Log.d("MainActivity", "POST_NOTIFICATIONS permission not granted. Requesting...")
                            ActivityCompat.requestPermissions(this, arrayOf(android.Manifest.permission.POST_NOTIFICATIONS), NOTIFICATION_PERMISSION_REQUEST_CODE)
                            // Do NOT start service here. It will be started in onRequestPermissionsResult if granted.
                        } else {
                            android.util.Log.d("MainActivity", "POST_NOTIFICATIONS permission already granted. Starting foreground service.")
                            startVpnShareService()
                        }
                    } else { // Below Android 13, permission is not needed at runtime
                        android.util.Log.d("MainActivity", "Android version < 13. Starting foreground service without POST_NOTIFICATIONS check.")
                        startVpnShareService()
                    }
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

    private fun startVpnShareService() {
        val serviceIntent = Intent(this, VpnShareService::class.java)
        ContextCompat.startForegroundService(this, serviceIntent) // Use ContextCompat
        android.util.Log.d("MainActivity", "Sent intent to start VpnShareService using ContextCompat.")
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == NOTIFICATION_PERMISSION_REQUEST_CODE) {
            if (grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
                android.util.Log.d("MainActivity", "POST_NOTIFICATIONS permission granted. Starting service.")
                startVpnShareService()
            } else {
                android.util.Log.d("MainActivity", "POST_NOTIFICATIONS permission denied. Service not started.")
                // Optionally, inform Flutter that permission was denied
            }
        }
    }
}

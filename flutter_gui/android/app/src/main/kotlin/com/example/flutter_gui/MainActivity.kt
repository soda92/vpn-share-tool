package com.example.flutter_gui

import android.app.Activity
import android.content.Intent
import android.os.Handler
import android.os.Looper
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.EventChannel
import android.os.Build
import android.content.pm.PackageManager
import androidx.core.app.ActivityCompat
import android.app.ActivityManager
import android.content.Context
import android.net.VpnService // Import VpnService
import androidx.core.content.ContextCompat // Import ContextCompat
import android.util.Log // Import Log

class MainActivity: FlutterActivity() {
    private val METHOD_CHANNEL = "vpn_share_tool/go_bridge"
    private val EVENT_CHANNEL = "vpn_share_tool/go_bridge_events"
    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel

    companion object {
        var eventSink: EventChannel.EventSink? = null
        private const val NOTIFICATION_PERMISSION_REQUEST_CODE = 1001
        private const val VPN_PERMISSION_REQUEST_CODE = 1002 // Define request code
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        methodChannel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, METHOD_CHANNEL)
        eventChannel = EventChannel(flutterEngine.dartExecutor.binaryMessenger, EVENT_CHANNEL)

        eventChannel.setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, sink: EventChannel.EventSink) {
                eventSink = sink
                Log.d("MainActivity", "EventChannel onListen: EventSink set.")
            }

            override fun onCancel(arguments: Any?) {
                eventSink = null
                Log.d("MainActivity", "EventChannel onCancel: EventSink cleared.")
            }
        })

        methodChannel.setMethodCallHandler {
            call, result ->
            when (call.method) {
                "startGoBackend" -> {
                    Log.d("MainActivity", "Received startGoBackend call from Dart.")
                    val dartCallback = object : mobile.EventCallback {
                        override fun onEvent(eventJSON: String?) {
                            if (eventJSON != null) {
                                Log.d("MainActivity", "Received event from Go: $eventJSON")
                                if (eventSink == null) {
                                    Log.e("MainActivity", "eventSink is null when trying to send event!")
                                } else {
                                    Log.d("MainActivity", "Sending event to Dart via EventChannel.")
                                    Handler(Looper.getMainLooper()).post {
                                        eventSink?.success(eventJSON)
                                    }
                                }
                            }
                        }
                    }
                    mobile.Mobile.setEventCallback(dartCallback)

                    Log.d("MainActivity", "Starting Go Mobile backend...")
                    val error = mobile.Mobile.start()
                    if (error != null && error.isNotEmpty()) {
                        Log.e("MainActivity", "Go backend failed to start: $error")
                        result.success(error)
                    } else {
                        result.success(null)
                    }
                }
                "startForegroundService" -> {
                    Log.d("MainActivity", "Received startForegroundService call from Dart.")
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                        if (ContextCompat.checkSelfPermission(this, android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                            Log.d("MainActivity", "POST_NOTIFICATIONS permission not granted. Requesting...")
                            ActivityCompat.requestPermissions(this, arrayOf(android.Manifest.permission.POST_NOTIFICATIONS), NOTIFICATION_PERMISSION_REQUEST_CODE)
                        } else {
                            Log.d("MainActivity", "POST_NOTIFICATIONS permission already granted. Starting foreground service.")
                            startVpnShareService(VpnShareService.ACTION_CONNECT) // Start service with CONNECT action
                        }
                    }
                    else {
                        Log.d("MainActivity", "Android version < 13. Starting foreground service without POST_NOTIFICATIONS check.")
                        startVpnShareService(VpnShareService.ACTION_CONNECT) // Start service with CONNECT action
                    }
                    result.success(null)
                }
                "stopForegroundService" -> {
                    Log.d("MainActivity", "Received stopForegroundService call from Dart.")
                    val serviceIntent = Intent(this, VpnShareService::class.java).apply {
                        action = VpnShareService.ACTION_DISCONNECT // Use DISCONNECT action
                    }
                    stopService(serviceIntent)
                    Log.d("MainActivity", "Sent intent to stop VpnShareService.")
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
                "hasNotificationPermission" -> {
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                        result.success(ContextCompat.checkSelfPermission(this, android.Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED)
                    }
                    else {
                        result.success(true)
                    }
                }
                "isForegroundServiceRunning" -> {
                    val manager = getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
                    @Suppress("DEPRECATION")
                    for (service in manager.getRunningServices(Integer.MAX_VALUE)) {
                        if (VpnShareService::class.java.name == service.service.className) {
                            result.success(true)
                            return@setMethodCallHandler
                        }
                    }
                    result.success(false)
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) { // Changed to non-nullable Intent
        super.onNewIntent(intent)
        if (intent.action == VpnShareService.ACTION_CONNECT) { // Removed null check
            // This intent is sent from VpnShareService to request VPN permission
            val prepareIntent = VpnService.prepare(this)
            if (prepareIntent != null) {
                startActivityForResult(prepareIntent, VPN_PERMISSION_REQUEST_CODE)
            } else {
                // Permission already granted, or user just came back from settings
                // Tell the service to proceed
                startVpnShareService(VpnShareService.ACTION_CONNECT)
            }
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_PERMISSION_REQUEST_CODE) {
            if (resultCode == Activity.RESULT_OK) {
                Log.d("MainActivity", "VPN permission granted by user.")
                // Permission granted, start the VpnShareService to proceed
                startVpnShareService(VpnShareService.ACTION_CONNECT)
            } else {
                Log.d("MainActivity", "VPN permission denied by user.")
                // Inform Dart that permission was denied
                eventSink?.success("{\"type\":\"error\",\"message\":\"VPN permission denied by user.\"}")
            }
        }
    }

    private fun startVpnShareService(action: String) {
        val serviceIntent = Intent(this, VpnShareService::class.java).apply {
            this.action = action
        }
        ContextCompat.startForegroundService(this, serviceIntent)
        Log.d("MainActivity", "Sent intent to start VpnShareService with action: $action")
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == NOTIFICATION_PERMISSION_REQUEST_CODE) {
            if (grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
                Log.d("MainActivity", "POST_NOTIFICATIONS permission granted. Starting service.")
                startVpnShareService(VpnShareService.ACTION_CONNECT) // Start service with CONNECT action
            } else {
                Log.d("MainActivity", "POST_NOTIFICATIONS permission denied. Service not started.")
                // Optionally, inform Flutter that permission was denied
            }
        }
    }
}
package com.example.flutter_gui

import android.app.ActivityManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel

class MainActivity: FlutterActivity() {
    private val METHOD_CHANNEL = "vpn_share_tool/go_bridge"
    private val EVENT_CHANNEL = "vpn_share_tool/go_bridge_events"
    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel
    private var finishReceiver: BroadcastReceiver? = null

    companion object {
        var eventSink: EventChannel.EventSink? = null
        private const val NOTIFICATION_PERMISSION_REQUEST_CODE = 1001
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        methodChannel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, METHOD_CHANNEL)
        eventChannel = EventChannel(flutterEngine.dartExecutor.binaryMessenger, EVENT_CHANNEL)

        setupFinishReceiver()

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
                    val serviceIntent = Intent(this, VpnShareService::class.java).apply {
                        action = VpnShareService.ACTION_EXIT_APP
                    }
                    startService(serviceIntent) // Use startService to trigger onStartCommand in the service
                    android.util.Log.d("MainActivity", "Sent intent to VpnShareService to trigger exit.")
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
                "startGoBackendWithPort" -> {
                    val port = call.argument<Int>("port")
                    if (port != null) {
                        android.util.Log.d("MainActivity", "Received startGoBackendWithPort call from Dart with port: $port")
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
                        mobile.Mobile.startGoBackendWithPort(port.toLong())
                        result.success(null)
                    } else {
                        result.error("INVALID_ARGUMENT", "Port argument is missing or not an integer", null)
                    }
                }
                "setDeviceIP" -> {
                    val ip = call.argument<String>("ip")
                    if (ip != null) {
                        mobile.Mobile.setDeviceIP(ip)
                        result.success(null)
                    } else {
                        result.error("INVALID_ARGUMENT", "IP argument is missing", null)
                    }
                }
                "setStoragePath" -> {
                    val path = call.argument<String>("path")
                    if (path != null) {
                        mobile.Mobile.setStoragePath(path)
                        result.success(null)
                    } else {
                        result.error("INVALID_ARGUMENT", "Path argument is missing", null)
                    }
                }
                "hasNotificationPermission" -> {
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                        result.success(ContextCompat.checkSelfPermission(this, android.Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED)
                    } else {
                        result.success(true) // Permission not needed below Android 13
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

    override fun onDestroy() {
        super.onDestroy()
        finishReceiver?.let {
            unregisterReceiver(it)
            finishReceiver = null
        }
    }

    private fun setupFinishReceiver() {
        finishReceiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context, intent: Intent) {
                if (intent.action == VpnShareService.ACTION_FINISH_MAIN_ACTIVITY) {
                    finish()
                }
            }
        }
        val intentFilter = IntentFilter(VpnShareService.ACTION_FINISH_MAIN_ACTIVITY)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(finishReceiver, intentFilter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            registerReceiver(finishReceiver, intentFilter)
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

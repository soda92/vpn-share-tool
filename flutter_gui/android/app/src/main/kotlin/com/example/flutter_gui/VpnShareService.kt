package com.example.flutter_gui

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.ParcelFileDescriptor
import android.util.Log
import mobile.Mobile
import mobile.EventCallback
import com.example.flutter_gui.MainActivity
import com.example.flutter_gui.R
import android.os.Process
import android.net.VpnService // Import VpnService

class VpnShareService : VpnService() { // Extend VpnService

    private val NOTIFICATION_CHANNEL_ID = "VpnShareServiceChannel"
    private val NOTIFICATION_ID = 101
    // We will not establish a VPN interface directly in this service
    // private var vpnInterface: ParcelFileDescriptor? = null

    companion object {
        const val ACTION_EXIT_APP = "EXIT_APP"
        const val ACTION_CONNECT = "CONNECT_VPN"
        const val ACTION_DISCONNECT = "DISCONNECT_VPN"
        const val VPN_PERMISSION_REQUEST_CODE = 1002
    }

    override fun onCreate() {
        super.onCreate()
        Log.d("VpnShareService", "Service onCreate")
        createNotificationChannel()
        val notification = createNotification()
        startForeground(NOTIFICATION_ID, notification)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d("VpnShareService", "Service onStartCommand, action: ${intent?.action}")

        when (intent?.action) {
            ACTION_EXIT_APP -> {
                Log.d("VpnShareService", "Received exit app intent. Stopping self and exiting app.")
                // No VPN interface to disconnect here, just stop the service
                stopSelf()
                Process.killProcess(Process.myPid())
            }
            ACTION_CONNECT -> {
                Log.d("VpnShareService", "Received connect VPN intent.")
                // Check if we have VPN permission
                val prepareIntent = prepare(this)
                if (prepareIntent != null) {
                    Log.d("VpnShareService", "VPN permission not granted. Requesting permission.")
                    // Send intent to MainActivity to request permission
                    val activityIntent = Intent(this, MainActivity::class.java).apply {
                        action = ACTION_CONNECT
                        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    }
                    startActivity(activityIntent)
                } else {
                    Log.d("VpnShareService", "VPN permission already granted. Signaling Go backend.")
                    // VPN permission is granted. Now, we can signal the Go backend
                    // to try and start its API server.
                    // The Go backend will attempt to bind its socket.
                    // We assume that having VpnService permission might allow this.
                    // No need to establish a VPN tunnel here.
                    // We can send an event to Dart to indicate success or failure of Go backend start.
                    val error = Mobile.start() // Call the Go Start() function again
                    if (error != null && error.isNotEmpty()) {
                        Log.e("VpnShareService", "Go backend failed to start after VPN permission: $error")
                        MainActivity.eventSink?.success("{\"type\":\"error\",\"message\":\"Go backend failed to start after VPN permission: $error\"}")
                    } else {
                        Log.d("VpnShareService", "Go backend started successfully after VPN permission.")
                        MainActivity.eventSink?.success("{\"type\":\"info\",\"message\":\"Go backend started successfully with VPN permission.\"}")
                    }
                }
            }
            ACTION_DISCONNECT -> {
                Log.d("VpnShareService", "Received disconnect VPN intent.")
                // No VPN interface to disconnect here, just stop the service
                stopSelf()
            }
            else -> {
                // If the system restarts the service, and we have VPN permission,
                // try to restart the Go backend.
                if (prepare(this) == null) { // Check if VPN permission is still granted
                    Log.d("VpnShareService", "Service restarted by system, VPN permission granted. Attempting to restart Go backend.")
                    val error = Mobile.start()
                    if (error != null && error.isNotEmpty()) {
                        Log.e("VpnShareService", "Go backend failed to restart after system restart: $error")
                        MainActivity.eventSink?.success("{\"type\":\"error\",\"message\":\"Go backend failed to restart after system restart: $error\"}")
                    } else {
                        Log.d("VpnShareService", "Go backend restarted successfully after system restart.")
                        MainActivity.eventSink?.success("{\"type\":\"info\",\"message\":\"Go backend restarted successfully after system restart.\"}")
                    }
                } else {
                    Log.d("VpnShareService", "Service restarted by system, VPN permission not granted. Cannot restart Go backend.")
                }
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        super.onDestroy()
        Log.d("VpnShareService", "Service onDestroy")
        // No VPN interface to disconnect here
        stopForeground(true)
    }

    override fun onBind(intent: Intent?): IBinder? {
        // VpnService does not typically bind, but if it does, it's usually for the system
        // to bind to it. For our purposes, we'll return null.
        return null
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val serviceChannel = NotificationChannel(
                NOTIFICATION_CHANNEL_ID,
                "VPN Share Service Channel",
                NotificationManager.IMPORTANCE_DEFAULT
            )
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(serviceChannel)
        }
    }

    private fun createNotification(): Notification {
        val exitAppIntent = Intent(this, VpnShareService::class.java).apply {
            action = ACTION_EXIT_APP
        }
        val pendingExitAppIntent: PendingIntent = PendingIntent.getService(
            this,
            0,
            exitAppIntent,
            PendingIntent.FLAG_CANCEL_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setContentTitle("VPN Share Tool")
            .setContentText("Sharing VPN connection in background...")
            .setSmallIcon(R.mipmap.ic_launcher)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Exit", pendingExitAppIntent)
            .build()
    }
}

package com.example.flutter_gui

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log
import com.example.flutter_gui.R // Import your app'''s R class

class VpnShareService : Service() {

    private val NOTIFICATION_CHANNEL_ID = "VpnShareServiceChannel"
    private val NOTIFICATION_ID = 101
    companion object {
        const val ACTION_EXIT_APP = "EXIT_APP"
        const val ACTION_FINISH_MAIN_ACTIVITY = "com.example.flutter_gui.FINISH_MAIN_ACTIVITY"
    }

    override fun onCreate() {
        super.onCreate()
        Log.d("VpnShareService", "Service onCreate")
        createNotificationChannel()
        val notification = createNotification()
        startForeground(NOTIFICATION_ID, notification)
        Log.d("VpnShareService", "Foreground service started.")
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d("VpnShareService", "Service onStartCommand with action: ${intent?.action}")
        if (intent?.action == ACTION_EXIT_APP) {
            Log.d("VpnShareService", "Received exit app intent. Telling MainActivity to finish and stopping service.")

            // Broadcast intent to tell MainActivity to finish
            val finishIntent = Intent(ACTION_FINISH_MAIN_ACTIVITY)
            sendBroadcast(finishIntent)

            stopSelf()
            return START_NOT_STICKY
        }
        // If the system kills the service, it will restart it.
        return START_STICKY
    }

    override fun onDestroy() {
        super.onDestroy()
        Log.d("VpnShareService", "Service onDestroy")
        stopForeground(true)
    }

    override fun onBind(intent: Intent?): IBinder? {
        return null // This is a started service, not a bound service
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
            Log.d("VpnShareService", "Notification channel created.")
        }
    }

    private fun createNotification(): Notification {
        // Intent to open the app when notification is tapped
        val notificationIntent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this,
            0,
            notificationIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        // Intent for the Exit button
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
            .setContentIntent(pendingIntent) // Open app on tap
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Exit", pendingExitAppIntent)
            .build()
    }
}
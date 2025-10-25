import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.Handler
import android.os.Looper
import android.util.Log
import mobile.Mobile
import mobile.EventCallback

class VpnShareService : Service() {

    private val NOTIFICATION_CHANNEL_ID = "VpnShareServiceChannel"
    private val NOTIFICATION_ID = 101

    override fun onCreate() {
        super.onCreate()
        Log.d("VpnShareService", "Service onCreate")
        createNotificationChannel()
        val notification = createNotification()
        startForeground(NOTIFICATION_ID, notification)

        // Initialize Go backend here
        Log.d("VpnShareService", "Starting Go Mobile backend...")
        Mobile.start()

        // Set event callback for Go Mobile
        val dartCallback = object : mobile.EventCallback {
            override fun onEvent(eventJSON: String?) {
                if (eventJSON != null) {
                    Log.d("VpnShareService", "Received event from Go: $eventJSON")
                    if (MainActivity.eventSink == null) {
                        Log.e("VpnShareService", "MainActivity.eventSink is null when trying to send event!")
                    } else {
                        Log.d("VpnShareService", "Sending event to Dart via EventChannel.")
                        // Ensure event is sent on the main thread
                        Handler(Looper.getMainLooper()).post {
                            MainActivity.eventSink?.success(eventJSON)
                        }
                    }
                }
            }
        }
        Mobile.setEventCallback(dartCallback)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d("VpnShareService", "Service onStartCommand")
        // If the system kills the service, it will restart it.
        return START_STICKY
    }

    override fun onDestroy() {
        super.onDestroy()
        Log.d("VpnShareService", "Service onDestroy")
        // Stop Go backend if necessary
        // Mobile.shutdown() // Assuming a shutdown method exists
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
                NotificationManager.IMPORTANCE_LOW
            )
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(serviceChannel)
        }
    }

    private fun createNotification(): Notification {
        return Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setContentTitle("VPN Share Tool")
            .setContentText("Sharing VPN connection in background...")
            .setSmallIcon(android.R.drawable.ic_dialog_info) // Replace with your app icon
            .build()
    }
}

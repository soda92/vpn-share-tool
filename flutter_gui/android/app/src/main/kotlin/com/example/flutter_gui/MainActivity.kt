package com.example.flutter_gui

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import mobile.Mobile

class MainActivity: FlutterActivity() {
    private val CHANNEL = "vpn_share_tool/go_bridge"

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL).setMethodCallHandler {
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
                "pollEvents" -> {
                    result.success(Mobile.pollEvents())
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }
}

package com.example.vpnsharetool;

import android.content.Context;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.net.NetworkCapabilities;
import java.net.InetAddress;

public class GetIP {
    public static String getIPAddress(Context context) {
        ConnectivityManager cm = (ConnectivityManager) context.getSystemService(Context.CONNECTIVITY_SERVICE);
        if (cm == null) {
            return "";
        }
        Network activeNetwork = cm.getActiveNetwork();
        if (activeNetwork == null) {
            return "";
        }
        LinkProperties linkProperties = cm.getLinkProperties(activeNetwork);
        if (linkProperties == null) {
            return "";
        }
        for (java.net.LinkAddress linkAddress : linkProperties.getLinkAddresses()) {
            InetAddress address = linkAddress.getAddress();
            if (address instanceof java.net.Inet4Address) {
                return address.getHostAddress();
            }
        }
        return "";
    }
}

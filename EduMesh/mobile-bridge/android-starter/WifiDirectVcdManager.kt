package com.schoolsync.vcd

import android.Manifest
import android.annotation.SuppressLint
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.net.NetworkInfo
import android.net.wifi.p2p.WifiP2pConfig
import android.net.wifi.p2p.WifiP2pDevice
import android.net.wifi.p2p.WifiP2pInfo
import android.net.wifi.p2p.WifiP2pManager
import androidx.core.content.ContextCompat

class WifiDirectVcdManager(
    private val context: Context,
    private val onState: (String) -> Unit
) {
    private val manager: WifiP2pManager? =
        context.getSystemService(Context.WIFI_P2P_SERVICE) as? WifiP2pManager
    private val channel: WifiP2pManager.Channel? =
        manager?.initialize(context, context.mainLooper, null)

    private var receiverRegistered = false

    private val receiver = object : BroadcastReceiver() {
        override fun onReceive(ctx: Context, intent: Intent) {
            when (intent.action) {
                WifiP2pManager.WIFI_P2P_STATE_CHANGED_ACTION -> {
                    onState("wifi_direct_state_changed")
                }
                WifiP2pManager.WIFI_P2P_PEERS_CHANGED_ACTION -> {
                    loadPeers()
                }
                WifiP2pManager.WIFI_P2P_CONNECTION_CHANGED_ACTION -> {
                    val info = intent.getParcelableExtra<NetworkInfo>(WifiP2pManager.EXTRA_NETWORK_INFO)
                    if (info?.isConnected == true) {
                        requestConnectionInfo()
                    }
                }
                WifiP2pManager.WIFI_P2P_THIS_DEVICE_CHANGED_ACTION -> {
                    onState("local_device_changed")
                }
            }
        }
    }

    fun start() {
        if (manager == null || channel == null) {
            onState("wifi_direct_unavailable")
            return
        }

        if (!hasNearbyPermissions()) {
            onState("missing_permissions")
            return
        }

        if (!receiverRegistered) {
            val filter = IntentFilter().apply {
                addAction(WifiP2pManager.WIFI_P2P_STATE_CHANGED_ACTION)
                addAction(WifiP2pManager.WIFI_P2P_PEERS_CHANGED_ACTION)
                addAction(WifiP2pManager.WIFI_P2P_CONNECTION_CHANGED_ACTION)
                addAction(WifiP2pManager.WIFI_P2P_THIS_DEVICE_CHANGED_ACTION)
            }
            context.registerReceiver(receiver, filter)
            receiverRegistered = true
        }

        discoverPeers()
    }

    fun stop() {
        if (receiverRegistered) {
            context.unregisterReceiver(receiver)
            receiverRegistered = false
        }
    }

    @SuppressLint("MissingPermission")
    private fun discoverPeers() {
        manager?.discoverPeers(channel, object : WifiP2pManager.ActionListener {
            override fun onSuccess() {
                onState("discover_started")
            }

            override fun onFailure(reason: Int) {
                onState("discover_failed_$reason")
            }
        })
    }

    @SuppressLint("MissingPermission")
    private fun loadPeers() {
        manager?.requestPeers(channel) { list ->
            if (list.deviceList.isEmpty()) {
                onState("no_peers_found")
                return@requestPeers
            }
            val best = choosePeer(list.deviceList.toList())
            if (best != null) {
                connect(best)
            } else {
                onState("no_eligible_peer")
            }
        }
    }

    @SuppressLint("MissingPermission")
    private fun connect(device: WifiP2pDevice) {
        val config = WifiP2pConfig().apply {
            deviceAddress = device.deviceAddress
        }
        manager?.connect(channel, config, object : WifiP2pManager.ActionListener {
            override fun onSuccess() {
                onState("connect_requested_${device.deviceAddress}")
            }

            override fun onFailure(reason: Int) {
                onState("connect_failed_$reason")
            }
        })
    }

    @SuppressLint("MissingPermission")
    private fun requestConnectionInfo() {
        manager?.requestConnectionInfo(channel) { info: WifiP2pInfo ->
            if (info.groupFormed) {
                val ownerIp = info.groupOwnerAddress?.hostAddress ?: "unknown"
                onState("group_ready_$ownerIp")
            } else {
                onState("group_not_formed")
            }
        }
    }

    private fun choosePeer(peers: List<WifiP2pDevice>): WifiP2pDevice? {
        return peers.firstOrNull()
    }

    private fun hasNearbyPermissions(): Boolean {
        val p1 = ContextCompat.checkSelfPermission(
            context,
            Manifest.permission.ACCESS_FINE_LOCATION
        ) == PackageManager.PERMISSION_GRANTED
        return p1
    }
}

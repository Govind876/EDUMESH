package com.schoolsync.vcdbridge.vcd

import org.json.JSONObject

data class BluetoothFallback(
    val enabled: Boolean,
    val mode: String
)

data class VcdPayload(
    val type: String,
    val downloadUrl: String,
    val serverIp: String,
    val port: Int,
    val ssid: String,
    val password: String,
    val bluetoothFallback: BluetoothFallback
)

object VcdPayloadParser {
    fun parse(raw: String): Result<VcdPayload> {
        return runCatching {
            val root = JSONObject(raw)
            val type = root.optString("type", "")
            require(type == "schoolsync-vcd") { "Unsupported payload type: $type" }
            val bt = root.optJSONObject("bluetoothFallback") ?: JSONObject()
            val payload = VcdPayload(
                type = type,
                downloadUrl = root.getString("downloadUrl"),
                serverIp = root.getString("serverIp"),
                port = root.getInt("port"),
                ssid = root.optString("ssid", ""),
                password = root.optString("password", ""),
                bluetoothFallback = BluetoothFallback(
                    enabled = bt.optBoolean("enabled", false),
                    mode = bt.optString("mode", "OPP")
                )
            )
            require(payload.downloadUrl.startsWith("http://") || payload.downloadUrl.startsWith("https://")) {
                "Invalid download URL"
            }
            payload
        }
    }
}

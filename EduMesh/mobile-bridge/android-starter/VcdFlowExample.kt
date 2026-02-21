package com.schoolsync.vcd

import android.content.Context
import android.util.Log

class VcdFlowExample(
    private val context: Context
) {
    private val wifiDirect = WifiDirectVcdManager(context) { state ->
        Log.d("VCD", "Wi-Fi Direct state: $state")
    }
    private val downloader = VcdDownloader(context)

    fun onQrScanned(raw: String, onDone: (Result<String>) -> Unit) {
        val parsed = VcdPayloadParser.parse(raw)
        if (parsed.isFailure) {
            onDone(Result.failure(parsed.exceptionOrNull() ?: IllegalArgumentException("Invalid QR payload")))
            return
        }

        val payload = parsed.getOrThrow()
        wifiDirect.start()

        // In real app: wait for connection callback indicating group/owner IP is ready,
        // then start download.
        val result = downloader.download(
            downloadUrl = payload.downloadUrl,
            outputName = "schoolsync.apk"
        )

        if (result.isSuccess) {
            onDone(Result.success(result.getOrThrow().file.absolutePath))
            return
        }

        if (payload.bluetoothFallback.enabled) {
            onDone(Result.failure(IllegalStateException("Wi-Fi Direct download failed. Switch to Bluetooth OPP fallback.")))
            return
        }

        onDone(Result.failure(result.exceptionOrNull() ?: IllegalStateException("Download failed")))
    }

    fun onStop() {
        wifiDirect.stop()
    }
}

package com.schoolsync.vcdbridge

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import android.widget.Button
import android.widget.EditText
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import com.google.zxing.client.android.Intents
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanOptions
import com.schoolsync.vcdbridge.vcd.ApkInstaller
import com.schoolsync.vcdbridge.vcd.VcdDownloader
import com.schoolsync.vcdbridge.vcd.VcdPayloadParser
import com.schoolsync.vcdbridge.vcd.WifiDirectVcdManager
import java.io.File
import java.util.concurrent.Executors

class MainActivity : ComponentActivity() {
    private lateinit var etPayload: EditText
    private lateinit var tvStatus: TextView
    private val io = Executors.newSingleThreadExecutor()
    private var downloadedApk: File? = null

    private val wifiDirect by lazy {
        WifiDirectVcdManager(this) { state ->
            runOnUiThread { setStatus("Wi-Fi Direct: $state") }
        }
    }
    private val downloader by lazy { VcdDownloader(this) }

    private val qrLauncher = registerForActivityResult(ScanContract()) { result ->
        val payload = result.contents.orEmpty()
        if (payload.isNotBlank()) {
            etPayload.setText(payload)
            setStatus("QR captured")
        } else {
            setStatus("QR scan canceled")
        }
    }

    private val permissionsLauncher =
        registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { map ->
            val denied = map.filterValues { !it }.keys
            if (denied.isEmpty()) setStatus("Permissions granted")
            else setStatus("Permissions missing: ${denied.joinToString()}")
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        etPayload = findViewById(R.id.etPayload)
        tvStatus = findViewById(R.id.tvStatus)
        val btnScanQr = findViewById<Button>(R.id.btnScanQr)
        val btnReceive = findViewById<Button>(R.id.btnReceive)
        val btnInstall = findViewById<Button>(R.id.btnInstall)

        requestRuntimePermissions()

        btnScanQr.setOnClickListener { startQrScan() }
        btnReceive.setOnClickListener { startReceive() }
        btnInstall.setOnClickListener {
            val apk = downloadedApk
            if (apk == null || !apk.exists()) {
                setStatus("No downloaded APK found")
                return@setOnClickListener
            }
            ApkInstaller.install(this, apk)
            setStatus("Install intent launched")
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        wifiDirect.stop()
        io.shutdownNow()
    }

    private fun startQrScan() {
        val options = ScanOptions().apply {
            setPrompt("Scan SchoolSync share QR")
            setDesiredBarcodeFormats(ScanOptions.QR_CODE)
            setBeepEnabled(false)
            setOrientationLocked(false)
            addExtra(Intents.Scan.MISSING_CAMERA_PERMISSION, true)
        }
        qrLauncher.launch(options)
    }

    private fun startReceive() {
        val raw = etPayload.text?.toString()?.trim().orEmpty()
        if (raw.isBlank()) {
            setStatus("Paste or scan QR payload first")
            return
        }
        val parsed = VcdPayloadParser.parse(raw)
        if (parsed.isFailure) {
            setStatus("Invalid payload: ${parsed.exceptionOrNull()?.message}")
            return
        }
        val payload = parsed.getOrThrow()

        wifiDirect.start()
        setStatus("Starting local download...")

        io.execute {
            val result = downloader.download(
                payload.downloadUrl,
                outputName = "schoolsync-received.apk"
            ) { downloaded, total ->
                runOnUiThread {
                    val percent = if (total > 0) (downloaded * 100 / total) else -1L
                    if (percent >= 0) setStatus("Downloading... $percent%")
                    else setStatus("Downloading... ${downloaded / 1024} KB")
                }
            }
            runOnUiThread {
                if (result.isSuccess) {
                    downloadedApk = result.getOrThrow().file
                    setStatus("Download complete: ${downloadedApk?.absolutePath}")
                } else {
                    val fallback = if (payload.bluetoothFallback.enabled) {
                        " Use Bluetooth OPP fallback."
                    } else {
                        ""
                    }
                    setStatus("Download failed: ${result.exceptionOrNull()?.message}.$fallback")
                }
            }
        }
    }

    private fun requestRuntimePermissions() {
        val requested = arrayOf(
            Manifest.permission.CAMERA,
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.NEARBY_WIFI_DEVICES,
            Manifest.permission.BLUETOOTH_CONNECT,
            Manifest.permission.BLUETOOTH_SCAN
        )
        val missing = requested.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }
        if (missing.isNotEmpty()) {
            permissionsLauncher.launch(missing.toTypedArray())
        }
    }

    private fun setStatus(message: String) {
        tvStatus.text = message
    }
}

package com.schoolsync.vcd

import android.content.Context
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL

class VcdDownloader(private val context: Context) {
    data class DownloadResult(
        val file: File,
        val bytes: Long
    )

    fun download(
        downloadUrl: String,
        outputName: String,
        onProgress: ((downloaded: Long, total: Long) -> Unit)? = null
    ): Result<DownloadResult> {
        return runCatching {
            val targetDir = context.getExternalFilesDir(null)
                ?: error("External files directory unavailable")
            val outFile = File(targetDir, outputName)

            val url = URL(downloadUrl)
            val conn = (url.openConnection() as HttpURLConnection).apply {
                connectTimeout = 10_000
                readTimeout = 60_000
                requestMethod = "GET"
                doInput = true
            }

            conn.connect()
            require(conn.responseCode in 200..299) { "HTTP ${conn.responseCode}" }

            val total = conn.contentLengthLong.takeIf { it > 0 } ?: -1L
            var downloaded = 0L

            conn.inputStream.use { input ->
                FileOutputStream(outFile).use { output ->
                    val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        output.write(buffer, 0, read)
                        downloaded += read
                        onProgress?.invoke(downloaded, total)
                    }
                    output.flush()
                }
            }
            conn.disconnect()

            DownloadResult(file = outFile, bytes = downloaded)
        }
    }
}

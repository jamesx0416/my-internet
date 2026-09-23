package dev.shizzi

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.os.SystemClock

class PhysicalUpstreams(private val context: Context) {

    data class Handles(val wifi: Long, val cellular: Long)

    private val connectivity = context.connectivityManager()
    private val lock = Object()

    @Volatile
    private var wifiNetwork: Network? = null

    @Volatile
    private var cellularNetwork: Network? = null

    private var wifiCallback: ConnectivityManager.NetworkCallback? = null
    private var cellularCallback: ConnectivityManager.NetworkCallback? = null

    fun acquire(timeoutMs: Long = ACQUIRE_TIMEOUT_MS): Handles {
        release()

        wifiCallback = request(NetworkCapabilities.TRANSPORT_WIFI) { network ->
            wifiNetwork = network
        }
        cellularCallback = request(NetworkCapabilities.TRANSPORT_CELLULAR) { network ->
            cellularNetwork = network
        }

        val deadline = SystemClock.elapsedRealtime() + timeoutMs
        synchronized(lock) {
            while (wifiNetwork == null || cellularNetwork == null) {
                val remaining = deadline - SystemClock.elapsedRealtime()
                if (remaining <= 0L) break
                lock.wait(remaining.coerceAtMost(POLL_SLICE_MS))
            }
        }

        val wifi = wifiNetwork
            ?: throw IllegalStateException("Wi-Fi upstream unavailable; connect the phone to Wi-Fi first")
        val cellular = cellularNetwork
            ?: throw IllegalStateException("cellular upstream unavailable; enable mobile data first")

        return Handles(
            wifi = wifi.networkHandle,
            cellular = cellular.networkHandle,
        )
    }

    private fun request(
        transport: Int,
        onAvailable: (Network) -> Unit,
    ): ConnectivityManager.NetworkCallback {
        val request = NetworkRequest.Builder()
            .addTransportType(transport)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            .build()

        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                onAvailable(network)
                synchronized(lock) { lock.notifyAll() }
            }

            override fun onLost(network: Network) {
                if (wifiNetwork == network) wifiNetwork = null
                if (cellularNetwork == network) cellularNetwork = null
                synchronized(lock) { lock.notifyAll() }
            }
        }

        connectivity.requestNetwork(request, callback)
        return callback
    }

    fun release() {
        wifiCallback?.let { callback ->
            runCatching { connectivity.unregisterNetworkCallback(callback) }
        }
        cellularCallback?.let { callback ->
            runCatching { connectivity.unregisterNetworkCallback(callback) }
        }
        wifiCallback = null
        cellularCallback = null
        wifiNetwork = null
        cellularNetwork = null
    }

    private companion object {
        const val ACQUIRE_TIMEOUT_MS = 15_000L
        const val POLL_SLICE_MS = 500L
    }
}

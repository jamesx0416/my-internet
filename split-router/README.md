# Split Router experiment

This branch stages a minimal first pass based on Shizzi.

Policy:

* default traffic: Wi-Fi
* x.com, twitter.com, twimg.com and t.co families: cellular
* no root required
* Shizuku provides the privileged tethering helper
* Tailscale is intentionally not part of this first test

The build workflow clones upstream Shizzi at a pinned commit, overlays the files in `split-router/replacements`, runs Go tests, and produces an arm64 debug APK.

## Phone test

1. Start Shizuku on the Xiaomi.
2. Connect the Xiaomi itself to Wi-Fi.
3. Keep mobile data enabled.
4. Install the APK over USB with ADB.
5. Open Split Router and grant its Shizuku request.
6. Start the session. The app will restart the hotspot.
7. Connect a second device to the Xiaomi hotspot.
8. For the first test, disable Secure DNS or DNS over HTTPS on that second device.
9. Browse a normal site, then x.com.
10. On the Mac, run:

   adb logcat | grep split-router

Normal destinations should log route=wifi. IPs learned from X DNS answers should log route=cellular.

## Wired install

Run:

    ./split-router/install-mac.sh /path/to/app-debug.apk

The script does not install Android Studio or an Android SDK. It only requires adb. If adb is missing, install Android platform tools separately.

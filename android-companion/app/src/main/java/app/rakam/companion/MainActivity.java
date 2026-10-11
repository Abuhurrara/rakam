package app.rakam.companion;

import android.Manifest;
import android.app.Activity;
import android.content.pm.PackageManager;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import org.json.JSONObject;

public final class MainActivity extends Activity {
    private LocalStore store;
    private TextView status;
    private EditText urlInput, senderInput, codeInput;
    private Button pairButton;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        store = new LocalStore(this);
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(24, 32, 24, 24);
        root.setBackgroundColor(0xfff6f2e8);
        TextView title = text("Rakam SMS companion", 24);
        root.addView(title);
        root.addView(text("Only new card-debit alerts from your chosen sender are parsed. The full SMS, phone number and OTPs are never sent to Rakam. Every purchase waits for your review before becoming an expense.", 15));
        root.addView(text("Rakam website (HTTPS)", 14));
        urlInput = input(store.baseUrl()); root.addView(urlInput);
        root.addView(text("Exact SMS sender", 14));
        senderInput = input(store.sender()); root.addView(senderInput);
        root.addView(text("One-time code from Rakam → More → Bank imports", 14));
        codeInput = input(""); root.addView(codeInput);
        pairButton = button("Pair this phone", v -> pair()); root.addView(pairButton);
        root.addView(button("Save website and sender", v -> saveSettings()));
        root.addView(button("Allow new SMS alerts", v -> requestSmsPermission()));
        root.addView(button("Sync pending alerts", v -> { SyncJob.schedule(this); updateStatus("Sync scheduled. Open Rakam to review alerts."); }));
        status = text("", 15); root.addView(status);
        ScrollView scroll = new ScrollView(this);
        scroll.addView(root);
        setContentView(scroll);
        updateStatus(store.token().isEmpty() ? "Not paired yet." : "Paired. " + store.pendingCount() + " alerts waiting on this phone. " + store.syncStatus());
    }

    private TextView text(String value, int size) {
        TextView view = new TextView(this);
        view.setText(value); view.setTextSize(size); view.setTextColor(0xff1d3029);
        view.setPadding(0, 12, 0, 12);
        return view;
    }
    private EditText input(String value) {
        EditText view = new EditText(this);
        view.setSingleLine(true); view.setText(value); view.setTextSize(16);
        return view;
    }
    private Button button(String label, View.OnClickListener click) {
        Button view = new Button(this); view.setText(label); view.setOnClickListener(click); return view;
    }
    private void updateStatus(String value) { status.setText(value); }

    private void saveSettings() {
        String base = urlInput.getText().toString().trim().replaceAll("/+$", "");
        String sender = senderInput.getText().toString().trim();
        if (!base.startsWith("https://") || sender.isEmpty()) {
            updateStatus("Enter an HTTPS website and an exact SMS sender."); return;
        }
        store.configure(base, sender);
        updateStatus("Saved. Only new alerts from " + sender + " will be considered.");
    }

    private void pair() {
        String base = urlInput.getText().toString().trim().replaceAll("/+$", "");
        String sender = senderInput.getText().toString().trim();
        String code = codeInput.getText().toString().trim();
        if (!base.startsWith("https://") || sender.isEmpty() || code.isEmpty()) {
            updateStatus("Enter an HTTPS website, sender and pairing code."); return;
        }
        pairButton.setEnabled(false);
        updateStatus("Pairing…");
        new Thread(() -> {
            try {
                JSONObject body = new JSONObject().put("code", code).put("name", "Android phone");
                HttpURLConnection conn = (HttpURLConnection) new URL(base + "/api/imports/pair").openConnection();
                JSONObject response;
                try {
                    conn.setRequestMethod("POST"); conn.setConnectTimeout(15_000); conn.setReadTimeout(30_000);
                    conn.setDoOutput(true); conn.setRequestProperty("Content-Type", "application/json");
                    try (OutputStream out = conn.getOutputStream()) { out.write(body.toString().getBytes(StandardCharsets.UTF_8)); }
                    if (conn.getResponseCode() != 200) throw new IllegalStateException("Pairing code was rejected or expired.");
                    try (InputStream in = conn.getInputStream(); ByteArrayOutputStream data = new ByteArrayOutputStream()) {
                        byte[] buffer = new byte[1024];
                        int count;
                        while ((count = in.read(buffer)) != -1) data.write(buffer, 0, count);
                        response = new JSONObject(data.toString("UTF-8"));
                    }
                } finally { conn.disconnect(); }
                store.configure(base, sender);
                store.saveToken(response.getString("token"));
                SyncJob.schedule(this);
                runOnUiThread(() -> { codeInput.setText(""); updateStatus("Paired. Now allow SMS alerts below."); });
            } catch (Exception ex) {
                runOnUiThread(() -> updateStatus("Could not pair: " + ex.getMessage()));
            } finally { runOnUiThread(() -> pairButton.setEnabled(true)); }
        }, "rakam-pair").start();
    }

    private void requestSmsPermission() {
        if (!getPackageManager().hasSystemFeature(PackageManager.FEATURE_TELEPHONY)) {
            updateStatus("This device cannot receive bank SMS alerts."); return;
        }
        if (checkSelfPermission(Manifest.permission.RECEIVE_SMS) == PackageManager.PERMISSION_GRANTED) {
            updateStatus("SMS permission is already allowed. Only new alerts from " + store.sender() + " will be used."); return;
        }
        requestPermissions(new String[]{Manifest.permission.RECEIVE_SMS}, 1);
    }

    @Override public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(requestCode, permissions, results);
        if (requestCode == 1) updateStatus(results.length > 0 && results[0] == PackageManager.PERMISSION_GRANTED
            ? "Ready. New matching alerts will sync when online." : "SMS permission denied. Import cannot run automatically.");
    }
}

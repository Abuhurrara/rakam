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
import java.text.DateFormat;
import java.util.Date;
import java.security.SecureRandom;
import org.json.JSONObject;

public final class MainActivity extends Activity {
    private LocalStore store;
    private TextView status;
    private TextView diagnostics;
    private EditText testSmsInput;
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
        status = text("", 15); root.addView(status);
        diagnostics = text("", 14); root.addView(diagnostics);
        root.addView(button("Refresh phone status", v -> refreshDiagnostics()));
        root.addView(text("Safe tests — nothing is uploaded or added to Expenses", 18));
        root.addView(button("Start dummy SMS test", v -> startSmsTest()));
        root.addView(text("For 15 minutes, the companion checks new SMS for that exact test code from any sender. No other SMS text is saved or sent. Ask another phone to send it to yours.", 14));
        root.addView(text("Check an old Meezan message without making a payment", 14));
        testSmsInput = new EditText(this);
        testSmsInput.setHint("Paste SMS text here; it is checked only on this phone");
        testSmsInput.setSingleLine(false);
        testSmsInput.setMinLines(2);
        testSmsInput.setMaxLines(4);
        testSmsInput.setSaveEnabled(false);
        testSmsInput.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO);
        root.addView(testSmsInput);
        root.addView(button("Check pasted message locally", v -> testPastedSms()));
        root.addView(button("Check Rakam connection (read-only)", v -> checkConnection()));
        root.addView(text("Rakam website (HTTPS)", 14));
        urlInput = input(store.baseUrl()); root.addView(urlInput);
        root.addView(text("Exact SMS sender", 14));
        senderInput = input(store.sender()); root.addView(senderInput);
        root.addView(text("One-time code from Rakam → More → Bank imports", 14));
        codeInput = input(""); root.addView(codeInput);
        pairButton = button("Pair this phone", v -> pair()); root.addView(pairButton);
        root.addView(button("Save website and sender", v -> saveSettings()));
        root.addView(button("Allow new SMS alerts", v -> requestSmsPermission()));
        root.addView(button("Sync pending alerts", v -> {
            if (store.pendingCount() == 0) {
                updateStatus("No alerts are waiting on this phone. Sync has nothing to send.");
                refreshDiagnostics();
                return;
            }
            try {
                SyncJob.schedule(this);
                updateStatus("Sync requested. Refresh phone status shortly, then open Rakam to review alerts.");
            } catch (RuntimeException ex) {
                updateStatus("Could not schedule sync: " + ex.getMessage());
            }
            refreshDiagnostics();
        }));
        ScrollView scroll = new ScrollView(this);
        scroll.addView(root);
        setContentView(scroll);
        refreshDiagnostics();
    }

    @Override protected void onResume() {
        super.onResume();
        if (diagnostics != null) refreshDiagnostics();
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

    private void refreshDiagnostics() {
        String paired = store.token().isEmpty() ? "Not paired" : "Paired";
        String permission = checkSelfPermission(Manifest.permission.RECEIVE_SMS) == PackageManager.PERMISSION_GRANTED
            ? "allowed" : "not allowed";
        diagnostics.setText("Phone status: " + paired + " · SMS permission " + permission
            + " · " + store.pendingCount() + " alerts waiting\n"
            + "Last SMS signal: " + timeOf(store.lastSmsAt()) + "\n"
            + "Last bank SMS from " + store.sender() + ": " + timeOf(store.lastBankSmsAt()) + "\n"
            + "Result: " + store.lastBankSmsResult() + "\n"
            + (store.activeSmsTestCode().isEmpty() ? "" : "Dummy SMS to send: RAKAM TEST " + store.activeSmsTestCode() + "\n")
            + "Dummy SMS result: " + store.smsTestResult() + "\n"
            + "Rakam connection: " + store.connectionStatus() + "\n"
            + "Sync: " + store.syncStatus()
            + "\nOnly times and results are saved here, never SMS text.");
    }

    private void startSmsTest() {
        String code = Integer.toString(100000 + new SecureRandom().nextInt(900000));
        try {
            store.beginSmsTest(code);
            updateStatus("For the next 15 minutes, ask another phone to send exactly: RAKAM TEST " + code);
        } catch (RuntimeException ex) {
            updateStatus("Could not start the SMS test on this phone.");
        }
        refreshDiagnostics();
    }

    private void testPastedSms() {
        String body = testSmsInput.getText().toString();
        testSmsInput.setText("");
        if (body.trim().isEmpty()) {
            updateStatus("Paste a Meezan debit SMS first. Nothing will be uploaded.");
            return;
        }
        MeezanParser.Alert alert = MeezanParser.parse(store.sender(), store.sender(), body, System.currentTimeMillis());
        updateStatus(alert == null
            ? "This text did not match the supported Meezan debit format. Nothing was uploaded."
            : "Format recognized: PKR " + alert.amount + " at " + alert.occurredAt + ". Nothing was uploaded.");
    }

    private void checkConnection() {
        String token = store.token();
        if (token.isEmpty()) {
            updateStatus("Pair this phone first. No data was sent.");
            return;
        }
        updateStatus("Checking Rakam connection…");
        new Thread(() -> {
            String result;
            try {
                HttpURLConnection conn = (HttpURLConnection) new URL(store.baseUrl() + "/api/imports/device-check").openConnection();
                try {
                    conn.setRequestMethod("GET");
                    conn.setConnectTimeout(15_000);
                    conn.setReadTimeout(15_000);
                    conn.setRequestProperty("Authorization", "Bearer " + token);
                    int statusCode = conn.getResponseCode();
                    if (statusCode == 200) result = "Connected. Rakam accepted this phone. No alert was sent.";
                    else if (statusCode == 401 || statusCode == 403) result = "Rakam rejected this pairing. Pair the phone again.";
                    else if (statusCode == 404) result = "Rakam has not deployed the new connection-check endpoint yet.";
                    else result = "Rakam connection check returned HTTP " + statusCode + ".";
                } finally { conn.disconnect(); }
            } catch (Exception ignored) {
                result = "Could not reach Rakam. Check internet and the website URL.";
            }
            String finalResult = result;
            store.setConnectionStatus(finalResult);
            runOnUiThread(() -> { updateStatus(finalResult); refreshDiagnostics(); });
        }, "rakam-device-check").start();
    }

    private String timeOf(long timestamp) {
        return timestamp == 0 ? "none yet"
            : DateFormat.getDateTimeInstance(DateFormat.MEDIUM, DateFormat.SHORT).format(new Date(timestamp));
    }

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
                String pairedStatus = "Paired. Now allow SMS alerts below.";
                try {
                    SyncJob.schedule(this);
                } catch (RuntimeException ex) {
                    pairedStatus = "Paired, but sync could not be scheduled: " + ex.getMessage();
                }
                String finalStatus = pairedStatus;
                runOnUiThread(() -> { codeInput.setText(""); updateStatus(finalStatus); refreshDiagnostics(); });
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
            updateStatus("SMS permission is already allowed. Only new alerts from " + store.sender() + " will be used.");
            refreshDiagnostics();
            return;
        }
        requestPermissions(new String[]{Manifest.permission.RECEIVE_SMS}, 1);
    }

    @Override public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(requestCode, permissions, results);
        if (requestCode == 1) updateStatus(results.length > 0 && results[0] == PackageManager.PERMISSION_GRANTED
            ? "Ready. New matching alerts will sync when online." : "SMS permission denied. Import cannot run automatically.");
        refreshDiagnostics();
    }
}

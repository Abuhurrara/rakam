package app.rakam.companion;

import android.Manifest;
import android.app.Activity;
import android.content.pm.PackageManager;
import android.graphics.Rect;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.view.inputmethod.InputMethodManager;
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
    private TextView summary, smsTestResult;
    private TextView diagnostics;
    private TextView pastedSmsResult, connectionResult;
    private EditText testSmsInput;
    private EditText urlInput, senderInput, codeInput;
    private Button pairButton, detailsButton;
    private LinearLayout detailsPanel;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        store = new LocalStore(this);
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(24, 24, 24, 24);
        root.setBackgroundColor(0xfff6f2e8);
        TextView title = text("Rakam SMS companion", 24);
        title.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        root.addView(title);
        root.addView(text("New Meezan debit alerts appear in Rakam for you to review.", 15));
        summary = text("", 17);
        summary.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        summary.setPadding(20, 16, 20, 16);
        GradientDrawable summaryBackground = new GradientDrawable();
        summaryBackground.setColor(0xffe0ece6);
        summaryBackground.setCornerRadius(18);
        summary.setBackground(summaryBackground);
        root.addView(summary);
        if ("Xiaomi".equalsIgnoreCase(Build.MANUFACTURER)) {
            root.addView(text("Xiaomi: allow Background autostart for this app in phone Settings, or alerts may be missed.", 14));
        }
        status = text("", 15); root.addView(status); updateStatus("");

        root.addView(section("1. Test that SMS reaches this phone"));
        root.addView(button("Start safe SMS test", v -> startSmsTest()));
        smsTestResult = text("", 15); root.addView(smsTestResult);

        root.addView(section("2. Test the bank message format"));
        root.addView(text("Paste an old alert. It stays on this phone and creates no expense.", 14));
        testSmsInput = new EditText(this);
        testSmsInput.setHint("Paste a Meezan debit SMS");
        testSmsInput.setSingleLine(false);
        testSmsInput.setMinLines(2);
        testSmsInput.setMaxLines(4);
        testSmsInput.setSaveEnabled(false);
        testSmsInput.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO);
        root.addView(testSmsInput);
        root.addView(button("Check message", v -> testPastedSms()));
        pastedSmsResult = text("", 15);
        root.addView(pastedSmsResult);

        root.addView(section("3. Check Rakam connection"));
        root.addView(button("Check connection", v -> checkConnection()));
        connectionResult = text(store.connectionStatus(), 15);
        root.addView(connectionResult);

        detailsButton = button("Show setup & details", v -> setDetailsVisible(detailsPanel.getVisibility() != View.VISIBLE));
        root.addView(detailsButton);
        detailsPanel = new LinearLayout(this);
        detailsPanel.setOrientation(LinearLayout.VERTICAL);
        detailsPanel.addView(text("Rakam website (HTTPS)", 14));
        urlInput = input(store.baseUrl()); detailsPanel.addView(urlInput);
        detailsPanel.addView(text("Exact SMS sender", 14));
        senderInput = input(store.sender()); detailsPanel.addView(senderInput);
        detailsPanel.addView(text("One-time code from Rakam → More → Bank imports", 14));
        codeInput = input(""); detailsPanel.addView(codeInput);
        pairButton = button("Pair this phone", v -> pair()); detailsPanel.addView(pairButton);
        detailsPanel.addView(button("Save website and sender", v -> saveSettings()));
        detailsPanel.addView(button("Allow new SMS alerts", v -> requestSmsPermission()));
        detailsPanel.addView(button("Sync pending alerts", v -> {
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
        detailsPanel.addView(button("Refresh phone status", v -> refreshDiagnostics()));
        diagnostics = text("", 14); detailsPanel.addView(diagnostics);
        root.addView(detailsPanel);
        setDetailsVisible(store.token().isEmpty()
            || checkSelfPermission(Manifest.permission.RECEIVE_SMS) != PackageManager.PERMISSION_GRANTED);
        ScrollView scroll = new ScrollView(this);
        scroll.setBackgroundColor(0xfff6f2e8);
        scroll.setClipToPadding(false);
        scroll.setOnApplyWindowInsetsListener((view, insets) -> {
            scroll.setPadding(0, insets.getSystemWindowInsetTop(), 0, insets.getSystemWindowInsetBottom());
            return insets;
        });
        getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
        getWindow().setStatusBarColor(0xfff6f2e8);
        getWindow().setNavigationBarColor(0xfff6f2e8);
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
    private TextView section(String value) {
        TextView view = text(value, 18);
        view.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        view.setPadding(0, 24, 0, 8);
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
    private void updateStatus(String value) {
        status.setText(value);
        status.setVisibility(value.isEmpty() ? View.GONE : View.VISIBLE);
    }

    private void setDetailsVisible(boolean visible) {
        detailsPanel.setVisibility(visible ? View.VISIBLE : View.GONE);
        detailsButton.setText(visible ? "Hide setup & details" : "Show setup & details");
    }

    private void refreshDiagnostics() {
        boolean paired = !store.token().isEmpty();
        boolean allowed = checkSelfPermission(Manifest.permission.RECEIVE_SMS) == PackageManager.PERMISSION_GRANTED;
        summary.setText(!paired ? "Setup needed: pair this phone" : !allowed
            ? "Setup needed: allow SMS alerts"
            : "Paired · SMS allowed · " + store.pendingCount() + " waiting");
        summary.setTextColor(paired && allowed ? 0xff206047 : 0xff9b3d31);
        String code = store.activeSmsTestCode();
        smsTestResult.setText(code.isEmpty() ? store.smsTestResult()
            : "Ask another phone to text you exactly:\nRAKAM TEST " + code
                + "\nThis code expires in 15 minutes. The test creates no expense.");
        String pairedLabel = paired ? "Paired" : "Not paired";
        String permission = allowed ? "allowed" : "not allowed";
        diagnostics.setText("Phone status: " + pairedLabel + " · SMS permission " + permission
            + " · " + store.pendingCount() + " alerts waiting\n"
            + "Last SMS signal: " + timeOf(store.lastSmsAt()) + "\n"
            + "Last bank SMS from " + store.sender() + ": " + timeOf(store.lastBankSmsAt()) + "\n"
            + "Result: " + store.lastBankSmsResult() + "\n"
            + "Dummy SMS result: " + store.smsTestResult() + "\n"
            + "Rakam connection: " + store.connectionStatus() + "\n"
            + "Sync: " + store.syncStatus()
            + "\nOnly times and results are saved here, never SMS text.");
    }

    private void startSmsTest() {
        String code = Integer.toString(100000 + new SecureRandom().nextInt(900000));
        try {
            store.beginSmsTest(code);
            updateStatus("");
        } catch (RuntimeException ex) {
            updateStatus("Could not start the SMS test on this phone.");
        }
        refreshDiagnostics();
        reveal(smsTestResult);
    }

    private void testPastedSms() {
        String body = testSmsInput.getText().toString();
        testSmsInput.setText("");
        InputMethodManager keyboard = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
        if (keyboard != null) keyboard.hideSoftInputFromWindow(testSmsInput.getWindowToken(), 0);
        testSmsInput.clearFocus();
        if (body.trim().isEmpty()) {
            showPastedSmsResult("Paste a Meezan debit SMS first. Nothing was uploaded.", false);
            return;
        }
        MeezanParser.Alert alert = MeezanParser.parse(store.sender(), store.sender(), body, System.currentTimeMillis());
        showPastedSmsResult(alert == null
            ? "Not recognized: this text did not match the Meezan debit format. Nothing was uploaded."
            : "Recognized: PKR " + alert.amount + " at " + alert.occurredAt + ". Nothing was uploaded.", alert != null);
    }

    private void showPastedSmsResult(String result, boolean recognized) {
        pastedSmsResult.setText(result);
        pastedSmsResult.setTextColor(recognized ? 0xff206047 : 0xff9b3d31);
        reveal(pastedSmsResult);
    }

    private void showConnectionResult(String result) {
        connectionResult.setText(result);
        reveal(connectionResult);
    }

    private void reveal(TextView view) {
        view.post(() -> view.requestRectangleOnScreen(new Rect(0, 0, view.getWidth(), view.getHeight()), true));
    }

    private void checkConnection() {
        String token = store.token();
        if (token.isEmpty()) {
            showConnectionResult("Pair this phone first. No data was sent.");
            return;
        }
        showConnectionResult("Checking Rakam connection…");
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
            runOnUiThread(() -> { showConnectionResult(finalResult); refreshDiagnostics(); });
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

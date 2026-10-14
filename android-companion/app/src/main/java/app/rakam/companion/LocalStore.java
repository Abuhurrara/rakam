package app.rakam.companion;

import android.content.Context;
import android.content.SharedPreferences;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Base64;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

final class LocalStore {
    private static final String PREFS = "rakam_companion";
    private static final String KEY_ALIAS = "rakam_import_token";
    private static final Object QUEUE_LOCK = new Object();
    private final SharedPreferences prefs;

    LocalStore(Context context) { prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE); }
    String baseUrl() { return prefs.getString("base_url", "https://rakam-five.vercel.app"); }
    String sender() { return prefs.getString("sender", "8079"); }
    void configure(String baseUrl, String sender) { prefs.edit().putString("base_url", baseUrl).putString("sender", sender).apply(); }
    String syncStatus() { return prefs.getString("sync_status", "No alerts synced yet."); }
    void setSyncStatus(String value) { prefs.edit().putString("sync_status", value).apply(); }
    long lastSmsAt() { return prefs.getLong("last_sms_at", 0); }
    long lastBankSmsAt() { return prefs.getLong("last_bank_sms_at", 0); }
    String lastBankSmsResult() { return prefs.getString("last_bank_sms_result", "No bank SMS seen by the companion yet."); }
    void noteSmsReceived() { prefs.edit().putLong("last_sms_at", System.currentTimeMillis()).commit(); }
    void noteBankSms(String result) {
        prefs.edit().putLong("last_bank_sms_at", System.currentTimeMillis())
            .putString("last_bank_sms_result", result).commit();
    }

    String token() {
        String encoded = prefs.getString("token", "");
        if (encoded.isEmpty()) return "";
        try {
            byte[] value = Base64.decode(encoded, Base64.NO_WRAP);
            byte[] iv = new byte[12];
            System.arraycopy(value, 0, iv, 0, iv.length);
            Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
            cipher.init(Cipher.DECRYPT_MODE, key(), new GCMParameterSpec(128, iv));
            return new String(cipher.doFinal(value, iv.length, value.length - iv.length), StandardCharsets.UTF_8);
        } catch (Exception ex) { return ""; }
    }

    void saveToken(String token) throws Exception {
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.ENCRYPT_MODE, key());
        byte[] iv = cipher.getIV();
        byte[] encrypted = cipher.doFinal(token.getBytes(StandardCharsets.UTF_8));
        byte[] combined = new byte[iv.length + encrypted.length];
        System.arraycopy(iv, 0, combined, 0, iv.length);
        System.arraycopy(encrypted, 0, combined, iv.length, encrypted.length);
        if (!prefs.edit().putString("token", Base64.encodeToString(combined, Base64.NO_WRAP)).commit()) throw new IllegalStateException("Could not save token");
    }

    private SecretKey key() throws Exception {
        KeyStore store = KeyStore.getInstance("AndroidKeyStore");
        store.load(null);
        SecretKey existing = (SecretKey) store.getKey(KEY_ALIAS, null);
        if (existing != null) return existing;
        KeyGenerator generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
        generator.init(new KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build());
        return generator.generateKey();
    }

    void enqueue(MeezanParser.Alert alert) throws JSONException {
        synchronized (QUEUE_LOCK) {
        JSONArray queue = queue();
        for (int i = 0; i < queue.length(); i++) if (alert.sourceId.equals(queue.getJSONObject(i).getString("source_id"))) return;
        JSONObject item = new JSONObject().put("source_id", alert.sourceId).put("amount", alert.amount).put("occurred_at", alert.occurredAt);
        queue.put(item);
        if (!prefs.edit().putString("queue", queue.toString()).commit()) throw new IllegalStateException("Could not queue alert");
        }
    }

    JSONArray queue() {
        try { return new JSONArray(prefs.getString("queue", "[]")); }
        catch (JSONException ex) { return new JSONArray(); }
    }
    int pendingCount() { return queue().length(); }

    void remove(String sourceId) throws JSONException {
        synchronized (QUEUE_LOCK) {
        JSONArray old = queue();
        JSONArray next = new JSONArray();
        for (int i = 0; i < old.length(); i++) {
            JSONObject item = old.getJSONObject(i);
            if (!sourceId.equals(item.getString("source_id"))) next.put(item);
        }
        if (!prefs.edit().putString("queue", next.toString()).commit()) throw new IllegalStateException("Could not update queue");
        }
    }
}

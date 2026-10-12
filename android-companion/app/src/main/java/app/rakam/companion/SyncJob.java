package app.rakam.companion;

import android.app.job.JobInfo;
import android.app.job.JobParameters;
import android.app.job.JobScheduler;
import android.app.job.JobService;
import android.content.ComponentName;
import android.content.Context;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import org.json.JSONArray;
import org.json.JSONObject;

/** Durable offline queue: Android retries when a network becomes available. */
public final class SyncJob extends JobService {
    private static final int JOB_ID = 8079;

    static void schedule(Context context) {
        JobScheduler scheduler = (JobScheduler) context.getSystemService(Context.JOB_SCHEDULER_SERVICE);
        JobInfo job = new JobInfo.Builder(JOB_ID, new ComponentName(context, SyncJob.class))
            .setRequiredNetworkType(JobInfo.NETWORK_TYPE_ANY)
            .setPersisted(true)
            .setBackoffCriteria(30_000, JobInfo.BACKOFF_POLICY_EXPONENTIAL)
            .build();
        if (scheduler.schedule(job) == JobScheduler.RESULT_FAILURE) {
            throw new IllegalStateException("Android could not schedule the background job");
        }
    }

    @Override public boolean onStartJob(JobParameters params) {
        new Thread(() -> {
            boolean retry = sync();
            jobFinished(params, retry);
        }, "rakam-import-sync").start();
        return true;
    }

    @Override public boolean onStopJob(JobParameters params) { return true; }

    private boolean sync() {
        LocalStore store = new LocalStore(this);
        String token = store.token();
        if (token.isEmpty()) {
            store.setSyncStatus("Phone is not paired. Pending alerts stay on this phone.");
            return false;
        }
        try {
            JSONArray items = store.queue();
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.getJSONObject(i);
                HttpURLConnection conn = (HttpURLConnection) new URL(store.baseUrl() + "/api/imports/sms").openConnection();
                try {
                    conn.setRequestMethod("POST");
                    conn.setConnectTimeout(15_000);
                    conn.setReadTimeout(30_000);
                    conn.setDoOutput(true);
                    conn.setRequestProperty("Content-Type", "application/json");
                    conn.setRequestProperty("Authorization", "Bearer " + token);
                    byte[] bytes = item.toString().getBytes(StandardCharsets.UTF_8);
                    try (OutputStream out = conn.getOutputStream()) { out.write(bytes); }
                    int status = conn.getResponseCode();
                    if (status != 200 && status != 201) {
                        if (status == 401 || status == 403) {
                            store.setSyncStatus("Pairing was rejected. Re-pair this phone, then tap Sync pending alerts.");
                            return false;
                        }
                        if (status >= 400 && status < 500) {
                            store.setSyncStatus("An alert was rejected (HTTP " + status + "). It remains on this phone for review.");
                            return false;
                        }
                        store.setSyncStatus("Rakam is temporarily unavailable. Will retry when possible.");
                        return true;
                    }
                    store.remove(item.getString("source_id"));
                } finally { conn.disconnect(); }
            }
            store.setSyncStatus("Synced. " + store.pendingCount() + " alerts remain on this phone.");
            return false;
        } catch (Exception ex) {
            store.setSyncStatus("Could not sync. Pending alerts are safe on this phone; will retry when possible.");
            return true;
        }
    }
}

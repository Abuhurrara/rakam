package app.rakam.companion;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.provider.Telephony;
import android.telephony.SmsMessage;

/** Receives new messages only; it never reads SMS history. */
public final class SmsReceiver extends BroadcastReceiver {
    @Override public void onReceive(Context context, Intent intent) {
        if (!Telephony.Sms.Intents.SMS_RECEIVED_ACTION.equals(intent.getAction())) return;
        SmsMessage[] parts = Telephony.Sms.Intents.getMessagesFromIntent(intent);
        if (parts == null || parts.length == 0) return;
        LocalStore store = new LocalStore(context);
        if (store.token().isEmpty()) return;
        String sender = parts[0].getOriginatingAddress();
        if (sender == null) return;
        StringBuilder body = new StringBuilder();
        for (SmsMessage part : parts) {
            if (!sender.equals(part.getOriginatingAddress())) return;
            body.append(part.getMessageBody());
        }
        MeezanParser.Alert alert = MeezanParser.parse(sender, store.sender(), body.toString(), parts[0].getTimestampMillis());
        if (alert == null) return;
        try { store.enqueue(alert); SyncJob.schedule(context); }
        catch (Exception ignored) { /* Do not leak SMS text to Android logs. */ }
    }
}

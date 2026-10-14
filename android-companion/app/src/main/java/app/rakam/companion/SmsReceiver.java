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
        LocalStore store = new LocalStore(context);
        store.noteSmsReceived();
        SmsMessage[] parts = Telephony.Sms.Intents.getMessagesFromIntent(intent);
        if (parts == null || parts.length == 0) return;
        String sender = parts[0].getOriginatingAddress();
        if (sender == null || !sender.equals(store.sender())) return;
        if (store.token().isEmpty()) {
            store.noteBankSms("Received, but this phone was not paired.");
            return;
        }
        StringBuilder body = new StringBuilder();
        for (SmsMessage part : parts) {
            if (!sender.equals(part.getOriginatingAddress())) {
                store.noteBankSms("Received, but message parts had different senders.");
                return;
            }
            body.append(part.getMessageBody());
        }
        MeezanParser.Alert alert = MeezanParser.parse(sender, store.sender(), body.toString(), parts[0].getTimestampMillis());
        if (alert == null) {
            store.noteBankSms("Received, but the debit format was not recognized.");
            return;
        }
        try {
            store.enqueue(alert);
            store.noteBankSms("Debit saved on this phone.");
            SyncJob.schedule(context);
        } catch (Exception ignored) {
            store.noteBankSms("Debit recognized, but saving or scheduling failed. Check alerts waiting.");
            // Never log the SMS text or exception: it may contain private information.
        }
    }
}

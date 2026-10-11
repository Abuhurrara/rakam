package app.rakam.companion;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeFormatterBuilder;
import java.time.format.DateTimeParseException;
import java.util.Locale;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Pure Java parser: no SMS body, sender, or phone number leaves the phone. */
public final class MeezanParser {
    private static final Pattern DEBIT = Pattern.compile(
        "^PKR\\s+([0-9][0-9,]*\\.[0-9]{2})\\s+has been debited at\\s+([0-9]{2}:[0-9]{2})\\s+on\\s+([0-9]{1,2}-[A-Za-z]{3}-[0-9]{4})(?:,|\\s|$)",
        Pattern.CASE_INSENSITIVE);
    private static final DateTimeFormatter DATE = new DateTimeFormatterBuilder()
        .parseCaseInsensitive().appendPattern("d-MMM-uuuu HH:mm").toFormatter(Locale.ENGLISH);
    private static final ZoneId KARACHI = ZoneId.of("Asia/Karachi");

    private MeezanParser() {}

    public static final class Alert {
        public final String amount;
        public final String occurredAt;
        public final String sourceId;
        Alert(String amount, String occurredAt, String sourceId) {
            this.amount = amount; this.occurredAt = occurredAt; this.sourceId = sourceId;
        }
    }

    public static Alert parse(String sender, String expectedSender, String body, long smsTimestamp) {
        if (sender == null || expectedSender == null || !sender.equals(expectedSender) || body == null) return null;
        Matcher match = DEBIT.matcher(body.trim());
        if (!match.find()) return null;
        try {
            String amount = match.group(1).replace(",", "");
            if (amount.substring(0, amount.indexOf('.')).length() > 13) return null;
            if (new BigDecimal(amount).signum() <= 0) return null;
            LocalDateTime local = LocalDateTime.parse(match.group(3) + " " + match.group(2), DATE);
            String occurredAt = local.atZone(KARACHI).toOffsetDateTime().toString();
            return new Alert(amount, occurredAt, sha256(sender + "\n" + body.trim() + "\n" + smsTimestamp));
        } catch (DateTimeParseException | NumberFormatException ex) {
            return null;
        }
    }

    private static String sha256(String input) {
        try {
            byte[] bytes = MessageDigest.getInstance("SHA-256").digest(input.getBytes(StandardCharsets.UTF_8));
            StringBuilder out = new StringBuilder(64);
            for (byte b : bytes) out.append(String.format(Locale.ROOT, "%02x", b & 0xff));
            return out.toString();
        } catch (Exception ex) { throw new IllegalStateException(ex); }
    }
}

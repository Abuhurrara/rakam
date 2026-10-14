import app.rakam.companion.MeezanParser;

public final class MeezanParserTest {
    public static void main(String[] args) {
        String sms = "PKR 506.00 has been debited at 00:10 on 12-Oct-2026, if you have not done this transaction, please inform us at 021111331331";
        MeezanParser.Alert alert = MeezanParser.parse("8079", "8079", sms, 12345L);
        check(alert != null, "sample SMS should parse");
        check("506.00".equals(alert.amount), "amount should remain a decimal string");
        check("2026-10-12T00:10+05:00".equals(alert.occurredAt), "Karachi date and time");
        check(alert.sourceId.equals(MeezanParser.parse("8079", "8079", sms, 12345L).sourceId), "stable duplicate ID");
        check(MeezanParser.parse("OTHER", "8079", sms, 12345L) == null, "wrong sender");
        check(MeezanParser.parse("8079", "8079", "Your OTP is 123456", 12345L) == null, "OTP ignored");
        check(MeezanParser.parse("8079", "8079", "PKR 506.00 has been credited at 00:10 on 12-Oct-2026", 12345L) == null, "credit ignored");
        check(MeezanParser.parse("8079", "8079", "PKR 0.00 has been debited at 00:10 on 12-Oct-2026", 12345L) == null, "zero ignored");
        check(MeezanParser.parse("8079", "8079", "PKR 1,506.75 has been debited at 23:59 on 01-Jan-2026", 12345L) != null, "comma amount");
        String purchase = "PKR 2,756.00 has been debited at 23:19 on 14-Oct-2026, If you have not done this transaction, please inform us at [redacted]";
        MeezanParser.Alert parsedPurchase = MeezanParser.parse("8079", "8079", purchase, 12345L);
        check(parsedPurchase != null, "reported shopping SMS should parse");
        check("2756.00".equals(parsedPurchase.amount), "reported shopping amount");
        check("2026-10-14T23:19+05:00".equals(parsedPurchase.occurredAt), "reported shopping time");
        System.out.println("Meezan parser checks passed");
    }
    private static void check(boolean ok, String message) { if (!ok) throw new AssertionError(message); }
}

package app.rakam.companion;

public final class SmsTestPhraseTest {
    public static void main(String[] args) {
        String code = "123456";
        String[] texts = {
            "RAKAM TEST 123456",
            " RAKAM TEST 123456 ",
            "RAKAM TEST 654321",
            "PKR 1.00 has been debited at 12:00 on 17-Oct-2026",
            "Your OTP is 123456"
        };
        boolean[] expected = {true, true, false, false, false};
        for (int i = 0; i < texts.length; i++) {
            if (SmsTestPhrase.matches(texts[i], code) != expected[i]) {
                throw new AssertionError("Unexpected result for test message " + i);
            }
        }
        System.out.println("Safe SMS test phrase checks passed");
    }
}

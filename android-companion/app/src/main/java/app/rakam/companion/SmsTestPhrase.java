package app.rakam.companion;

/** A deliberately separate test command that can never become an import. */
final class SmsTestPhrase {
    private SmsTestPhrase() {}

    static boolean matches(String body, String code) {
        return body != null && code != null && code.matches("[0-9]{6}")
            && body.trim().equals("RAKAM TEST " + code);
    }
}

package com.deervery.rakusikaliteclient;
import org.junit.Test;
import static org.junit.Assert.*;

public class EndpointPolicyTest {
    @Test public void acceptsLiteAndTailnet() {
        assertEquals("https://rakusika.com/login", EndpointPolicy.web("https://rakusika.com/login"));
        assertEquals("https://glwin.zapus-opah.ts.net:8445", EndpointPolicy.hub("https://glwin.zapus-opah.ts.net:8445/"));
    }
    @Test public void blocksSchemeCredentialsAndLookalikes() {
        for (String value : new String[]{"http://rakusika.com", "javascript:alert(1)", "https://rakusika.com.evil.test", "https://user:pass@rakusika.com", "https://glwin.zapus-opah.ts.net.evil.test"}) {
            assertThrows(IllegalArgumentException.class, () -> EndpointPolicy.web(value));
        }
    }
    @Test public void originIncludesPort() {
        assertTrue(EndpointPolicy.sameOrigin("https://rakusika.com:443/a", "https://rakusika.com/b"));
        assertFalse(EndpointPolicy.sameOrigin("https://glwin.zapus-opah.ts.net:8445", "https://glwin.zapus-opah.ts.net:8444"));
    }
    @Test public void hubRejectsQueryAndNormalizes() {
        assertThrows(IllegalArgumentException.class, () -> EndpointPolicy.hub("https://hub.test/?token=x"));
        assertEquals("", EndpointPolicy.hub(""));
        assertEquals("https://hub.test/hub", EndpointPolicy.hub(" https://hub.test/hub/ "));
    }
}

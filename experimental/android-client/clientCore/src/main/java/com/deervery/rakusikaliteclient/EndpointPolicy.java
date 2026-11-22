package com.deervery.rakusikaliteclient;

import java.net.URI;
import java.util.Set;
import java.util.HashSet;
import java.util.Arrays;

/** No native privileged JS bridge. Only explicit trusted web origins run in the shell. */
public final class EndpointPolicy {
    private static final Set<String> WEB_HOSTS = new HashSet<>(Arrays.asList("rakusika.com", "www.rakusika.com", "raku-sika-lite.web.app", "raku-sika-lite.firebaseapp.com"));
    private EndpointPolicy() {}
    public static String web(String value) {
        URI uri = secure(value);
        if (!WEB_HOSTS.contains(uri.getHost()) && !uri.getHost().endsWith(".zapus-opah.ts.net"))
            throw new IllegalArgumentException("ラクシカまたはokyoheのTailscale HTTPS URLを指定してください。");
        return uri.toASCIIString();
    }
    public static String hub(String value) {
        if (value == null || value.trim().isEmpty()) return "";
        URI uri = secure(value);
        if (uri.getQuery() != null || uri.getFragment() != null)
            throw new IllegalArgumentException("ハブURLにクエリーやフラグメントは指定できません。");
        return uri.toASCIIString().replaceAll("/+$", "");
    }
    private static URI secure(String value) {
        URI uri;
        try { uri = URI.create(value.trim()).normalize(); }
        catch (RuntimeException e) { throw new IllegalArgumentException("HTTPS URLを確認してください。"); }
        if (!"https".equals(uri.getScheme()) || uri.getHost() == null || uri.getUserInfo() != null || uri.getPort() == 0 || uri.getPort() > 65535)
            throw new IllegalArgumentException("ユーザー名・パスワードを含まないHTTPS URLが必要です。");
        return uri;
    }
    public static String origin(String value) {
        URI uri = secure(value);
        return "https://" + uri.getHost() + (uri.getPort() == -1 || uri.getPort() == 443 ? "" : ":" + uri.getPort());
    }
    public static boolean sameOrigin(String first, String second) {
        try { return origin(first).equals(origin(second)); } catch (RuntimeException e) { return false; }
    }
}

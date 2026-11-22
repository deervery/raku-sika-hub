package com.deervery.rakusikaliteclient;

/** One explicitly selected HID device; no partial replay after timeout/overflow. */
public final class ScannerBuffer {
    private final StringBuilder buffer=new StringBuilder();
    private long last=-1;
    private boolean overflow;
    public void clear(){buffer.setLength(0);last=-1;overflow=false;}
    public String accept(int codePoint,boolean enter,long time){
        if(last>=0&&(time<last||time-last>500)){buffer.setLength(0);overflow=true;}
        last=time;
        if(enter){String result=overflow||buffer.length()==0?null:buffer.toString();clear();return result;}
        if(overflow)return null;
        if(codePoint<=0||!Character.isValidCodePoint(codePoint)||Character.isISOControl(codePoint))return null;
        buffer.appendCodePoint(codePoint);
        if(buffer.length()>2048){buffer.setLength(0);overflow=true;}
        return null;
    }
}

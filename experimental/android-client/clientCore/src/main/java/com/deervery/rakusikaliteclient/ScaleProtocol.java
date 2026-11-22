package com.deervery.rakusikaliteclient;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** A&D USB serial 2400/7E1. Invalid or overload data is never a valid zero. */
public final class ScaleProtocol {
    private static final Pattern WEIGHT=Pattern.compile("^(ST|US),\\s*([+-]?\\d+(?:\\.\\d+)?)\\s*(kg|g)$");
    public static final class Reading {
        public final BigDecimal value;
        public final String unit;
        public final boolean stable;
        Reading(BigDecimal value,String unit,boolean stable){this.value=value;this.unit=unit;this.stable=stable;}
    }
    public static Reading parse(String raw){
        Matcher m=WEIGHT.matcher(raw.trim());
        if(!m.matches())throw new IllegalArgumentException("過負荷または未対応の計量応答です。値は確定できません。");
        return new Reading(new BigDecimal(m.group(2)),m.group(3),m.group(1).equals("ST"));
    }
    public static byte[] command(String command){
        if(!command.equals("Q")&&!command.equals("T")&&!command.equals("R"))throw new IllegalArgumentException("Unknown scale command");
        return (command+"\r\n").getBytes(StandardCharsets.US_ASCII);
    }
    public static boolean ack(String command,String raw){
        String header=raw.trim().split(",",2)[0];
        return command.equals("T")?(header.equals("QT")||header.equals("TA")):command.equals("R")&&header.equals("ZR");
    }
    public static final class Framer {
        private final StringBuilder pending=new StringBuilder();
        public List<String> accept(byte[] bytes,int count){
            List<String> result=new ArrayList<>();
            for(int i=0;i<count;i++){
                int b=bytes[i]&255;
                if(b=='\n') {String line=pending.toString().trim();pending.setLength(0);if(!line.isEmpty())result.add(line);}
                else if(b!='\r') {
                    if(b<32||b>126||pending.length()>=128){pending.setLength(0);throw new IllegalArgumentException("計量データが破損しています。");}
                    pending.append((char)b);
                }
            }
            return result;
        }
    }
}

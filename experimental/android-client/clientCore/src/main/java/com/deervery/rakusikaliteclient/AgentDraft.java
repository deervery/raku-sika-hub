package com.deervery.rakusikaliteclient;

import java.util.*;
import java.math.BigDecimal;
import java.text.Normalizer;
import java.util.regex.*;

/** Local input draft only. UI navigation never registers or prints a record. */
public final class AgentDraft {
    public static final class Offer {
        public final long revision,time;public final Map<String,String> values;
        Offer(long revision,long time,Map<String,String> values){this.revision=revision;this.time=time;this.values=Collections.unmodifiableMap(new LinkedHashMap<>(values));}
    }
    private final Map<String,String> values=new LinkedHashMap<>();private Map<String,String> undo;private long revision;
    public AgentDraft(){values.put("individual","");values.put("part","");values.put("weight","");}
    public Map<String,String> values(){return Collections.unmodifiableMap(new LinkedHashMap<>(values));}
    public long revision(){return revision;}
    public void restore(Map<String,String> saved){for(String key:values.keySet())values.put(key,saved.getOrDefault(key,""));revision++;}
    public Offer propose(String text,long base,long now){
        if(base!=revision)throw new IllegalStateException("入力が変わりました。もう一度話してください。");
        String s=Normalizer.normalize(text,Normalizer.Form.NFKC).trim().replaceFirst("^訂正[、,\\s]*","");
        Map<String,String> next=new LinkedHashMap<>();
        for(String clause:s.split("[、,。\\n]+")){
            clause=clause.trim();if(clause.isEmpty())continue;
            Matcher id=Pattern.compile("個体(?:番号|ID)?\\s*([A-Za-z0-9][A-Za-z0-9-]{0,31})",Pattern.CASE_INSENSITIVE).matcher(clause);
            Matcher part=Pattern.compile("(?:部位(?:は)?|次(?:は)?)?\\s*(モモ|もも|ロース|バラ|ばら|スネ|すね)").matcher(clause);
            if(id.matches())next.put("individual",id.group(1));
            else if(part.matches()){String p=part.group(1);next.put("part",p.equals("もも")?"モモ":p.equals("ばら")?"バラ":p.equals("すね")?"スネ":p);}
            else{
                String weight=clause.replaceFirst("^重量(?:は)?\\s*","").replace("キログラム","kg").replace("キロ","kg").replace("グラム","g");
                Matcher m=Pattern.compile("([+]?\\d{1,6}(?:\\.\\d{1,3})?)\\s*(kg|g)").matcher(weight);
                if(!m.matches())throw new IllegalArgumentException("個体番号・部位・重量のどれを入力しますか？");
                BigDecimal v=new BigDecimal(m.group(1));if(m.group(2).equals("g"))v=v.movePointLeft(3);
                next.put("weight",v.stripTrailingZeros().toPlainString());
            }
        }
        if(next.isEmpty())throw new IllegalArgumentException("聞き取れませんでした。もう一度お願いします。");
        return new Offer(base,now,next);
    }
    public Offer weight(String kg,long now){new BigDecimal(kg);return new Offer(revision,now,Collections.singletonMap("weight",kg));}
    public void accept(Offer offer,long now){
        if(offer.revision!=revision||now<offer.time||now-offer.time>30000)throw new IllegalStateException("候補が古くなりました。もう一度入力してください。");
        undo=new LinkedHashMap<>(values);
        if((offer.values.containsKey("individual")&&!offer.values.get("individual").equals(values.get("individual")))||
           (offer.values.containsKey("part")&&!offer.values.get("part").equals(values.get("part"))))values.put("weight","");
        values.putAll(offer.values);revision++;
    }
    public boolean undo(){if(undo==null)return false;values.clear();values.putAll(undo);undo=null;revision++;return true;}
}

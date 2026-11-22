package com.deervery.rakusikaliteclient;

import android.content.Context;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.content.res.ColorStateList;
import android.graphics.drawable.RippleDrawable;
import android.view.Gravity;
import android.widget.*;

final class StationStyle {
    static final int PAPER=0xfff8fafc,INK=0xff1e3a5f,MUTED=0xff4b5563,ACCENT=0xff4a6fa5,SOFT=0xffeaf0f8,LINE=0xffe5e7eb,WHITE=0xffffffff;
    static int dp(Context c,float value){return Math.round(value*c.getResources().getDisplayMetrics().density);}
    static GradientDrawable shape(Context c,int color,int radius){GradientDrawable d=new GradientDrawable();d.setColor(color);d.setCornerRadius(dp(c,radius));return d;}
    static TextView text(Context c,String value,int size,boolean bold){
        TextView v=new TextView(c);v.setText(value);v.setTextColor(INK);v.setTextSize(size);
        v.setFontFeatureSettings("kern");v.setTypeface(Typeface.create("sans-serif",Typeface.BOLD));return v;
    }
    static Button button(Context c,String value,boolean primary,Runnable action){
        Button b=new Button(c);b.setText(value);b.setTextSize(16);b.setAllCaps(false);b.setTypeface(Typeface.create("sans-serif",Typeface.BOLD));
        b.setTextColor(primary?WHITE:INK);b.setMinHeight(dp(c,52));b.setMinimumHeight(dp(c,52));b.setMinWidth(0);b.setMinimumWidth(0);
        b.setPadding(dp(c,16),dp(c,10),dp(c,16),dp(c,10));
        b.setBackground(new RippleDrawable(ColorStateList.valueOf(0x184a6fa5),shape(c,primary?ACCENT:SOFT,18),null));
        b.setOnClickListener(v->action.run());return b;
    }
    static LinearLayout column(Context c){LinearLayout l=new LinearLayout(c);l.setOrientation(LinearLayout.VERTICAL);return l;}
    static void space(LinearLayout l,int dp){android.view.View v=new android.view.View(l.getContext());l.addView(v,new LinearLayout.LayoutParams(1,dp(l.getContext(),dp)));}
}

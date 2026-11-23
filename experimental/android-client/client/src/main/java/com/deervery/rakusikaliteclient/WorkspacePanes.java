package com.deervery.rakusikaliteclient;
import android.content.Context;
import android.view.View;
import android.widget.FrameLayout;
import android.animation.ValueAnimator;

/** Retains both view instances and drafts; only visibility changes during navigation. */
final class WorkspacePanes extends FrameLayout {
    private final View work,agent;private final android.widget.TextView verifying;private boolean inAgent;private int generation;
    WorkspacePanes(Context context,View work,View agent){
        super(context);this.work=work;this.agent=agent;addView(work,new LayoutParams(-1,-1));addView(agent,new LayoutParams(-1,-1));agent.setVisibility(GONE);verifying=StationStyle.text(context,"ログインを確認しています…",20,true);verifying.setGravity(android.view.Gravity.CENTER);verifying.setBackgroundColor(StationStyle.PAPER);verifying.setClickable(true);addView(verifying,new LayoutParams(-1,-1));verifying.setVisibility(GONE);
    }
    void showVerifying(boolean value){verifying.setVisibility(value?VISIBLE:GONE);if(value)verifying.bringToFront();}
    void showAgent(boolean value){
        if(inAgent==value)return;inAgent=value;int token=++generation;
        View incoming=value?agent:work,outgoing=value?work:agent;
        incoming.animate().cancel();outgoing.animate().cancel();outgoing.setEnabled(false);incoming.setEnabled(true);
        outgoing.setVisibility(GONE);incoming.setVisibility(VISIBLE);incoming.bringToFront();
        if(!ValueAnimator.areAnimatorsEnabled()){incoming.setAlpha(1);outgoing.setVisibility(GONE);return;}
        incoming.setAlpha(1);incoming.setTranslationY(StationStyle.dp(getContext(),6));
        incoming.animate().alpha(1).translationY(0).setDuration(220).withEndAction(()->{if(generation==token)outgoing.setVisibility(GONE);}).start();
    }
}

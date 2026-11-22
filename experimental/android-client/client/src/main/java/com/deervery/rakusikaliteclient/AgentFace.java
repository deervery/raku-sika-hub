package com.deervery.rakusikaliteclient;
import android.content.Context;
import android.graphics.*;
import android.view.View;
import android.animation.ValueAnimator;

/** Eyes, mouth and cheeks only. Motion stops while hidden and respects reduced motion. */
final class AgentFace extends View {
    private final Paint p=new Paint(Paint.ANTI_ALIAS_FLAG);
    private ValueAnimator animator;private float phase;private boolean listening;
    AgentFace(Context c){super(c);setFocusable(true);setClickable(true);setContentDescription("タップして話す。長押しで入力や計量の操作を選べます");}
    void listening(boolean value){listening=value;setContentDescription(value?"聞き取りを停止する":"タップして話す。長押しで入力や計量の操作を選べます");invalidate();}
    private void motion(){
        if(animator!=null){animator.cancel();animator=null;}
        if(!isShown()||getWindowVisibility()!=VISIBLE||!ValueAnimator.areAnimatorsEnabled())return;
        animator=ValueAnimator.ofFloat(0,1);animator.setDuration(4200);animator.setRepeatCount(ValueAnimator.INFINITE);
        animator.addUpdateListener(a->{phase=(float)a.getAnimatedValue();invalidate();});animator.start();
    }
    @Override protected void onAttachedToWindow(){super.onAttachedToWindow();motion();}
    @Override protected void onDetachedFromWindow(){if(animator!=null)animator.cancel();super.onDetachedFromWindow();}
    @Override protected void onVisibilityChanged(View changed,int visibility){super.onVisibilityChanged(changed,visibility);motion();}
    @Override protected void onWindowVisibilityChanged(int visibility){super.onWindowVisibilityChanged(visibility);motion();}
    @Override protected void onDraw(Canvas c){
        super.onDraw(c);float w=Math.min(getWidth()*.76f,getHeight()*1.48f),cx=getWidth()/2f,cy=getHeight()*.48f;
        float eyeY=cy-w*.07f,blink=phase>.92f&&phase<.97f?.16f:1f;
        p.setColor(StationStyle.INK);p.setStyle(Paint.Style.FILL);
        for(int sign:new int[]{-1,1}){
            float x=cx+sign*w*.21f;
            c.drawRoundRect(x-w*.037f,eyeY-w*.065f*blink,x+w*.037f,eyeY+w*.065f*blink,w*.04f,w*.04f,p);
        }
        p.setColor(0xffedc1b4);p.setAlpha(210);
        c.drawOval(cx-w*.36f,cy+w*.04f,cx-w*.20f,cy+w*.095f,p);
        c.drawOval(cx+w*.20f,cy+w*.04f,cx+w*.36f,cy+w*.095f,p);p.setAlpha(255);
        p.setColor(StationStyle.INK);p.setStrokeWidth(w*.023f);p.setStrokeCap(Paint.Cap.ROUND);
        if(listening){p.setStyle(Paint.Style.FILL);float a=.85f+.15f*(float)Math.sin(phase*Math.PI*8);c.drawOval(cx-w*.045f,cy+w*.045f,cx+w*.045f,cy+w*(.10f+.055f*a),p);}
        else {p.setStyle(Paint.Style.STROKE);Path smile=new Path();smile.moveTo(cx-w*.07f,cy+w*.065f);smile.quadTo(cx,cy+w*.14f,cx+w*.07f,cy+w*.065f);c.drawPath(smile,p);}
        p.setStyle(Paint.Style.FILL);
    }
}

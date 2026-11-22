package com.deervery.rakusikaliteclient;

import java.io.IOException;
import java.util.function.BooleanSupplier;
import java.util.function.LongSupplier;

/** One fresh transaction. No retries of tare/zero, and no cached readings. */
public final class ScaleExchange {
    public interface Wire {int read(byte[] buffer,int timeout) throws IOException;void write(byte[] data,int timeout) throws IOException;}
    public static ScaleProtocol.Reading run(Wire wire,String command,LongSupplier clock,BooleanSupplier valid)throws IOException {
        byte[] request=ScaleProtocol.command(command),buffer=new byte[256];
        long drainEnd=clock.getAsLong()+250;
        while(clock.getAsLong()<drainEnd){check(valid);if(wire.read(buffer,40)==0)break;}
        check(valid);wire.write(request,800);
        long end=clock.getAsLong()+2500;
        ScaleProtocol.Framer framer=new ScaleProtocol.Framer();
        while(clock.getAsLong()<end){
            check(valid);
            int count=wire.read(buffer,200);check(valid);
            for(String line:framer.accept(buffer,count)){
                if(command.equals("Q")){
                    if(ScaleProtocol.ack("T",line)||ScaleProtocol.ack("R",line))continue;
                    return ScaleProtocol.parse(line);
                }
                if(ScaleProtocol.ack(command,line))return null;
                // Streaming weights do not acknowledge a tare/zero command.
            }
        }
        throw new IOException(command.equals("Q")?"計量応答がありません。2400/7E1とコマンド受付設定を確認してください。":"操作結果を確認できません。本体を確認してください。自動再送はしません。");
    }
    private static void check(BooleanSupplier valid)throws IOException {if(!valid.getAsBoolean())throw new IOException("接続または画面が変わったため、計量結果を破棄しました。");}
}

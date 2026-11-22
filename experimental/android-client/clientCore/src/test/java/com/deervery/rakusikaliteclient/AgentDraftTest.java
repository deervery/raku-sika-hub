package com.deervery.rakusikaliteclient;
import org.junit.Test;
import static org.junit.Assert.*;
public class AgentDraftTest {
    @Test public void speechIsProposalUntilAccepted(){AgentDraft d=new AgentDraft();AgentDraft.Offer o=d.propose("個体001、モモ、70グラム",0,100);assertEquals("",d.values().get("weight"));d.accept(o,101);assertEquals("001",d.values().get("individual"));assertEquals("0.07",d.values().get("weight"));}
    @Test public void staleSpeechCannotOverwriteNewerValue(){AgentDraft d=new AgentDraft();AgentDraft.Offer old=d.propose("モモ",0,100);d.accept(d.propose("ロース",0,101),102);assertThrows(IllegalStateException.class,()->d.accept(old,103));assertThrows(IllegalStateException.class,()->d.propose("1kg",0,105));}
    @Test public void changingTargetClearsOldWeightAndUndoRestores(){AgentDraft d=new AgentDraft();d.accept(d.propose("モモ、1kg",0,100),101);d.accept(d.propose("ロース",1,200),201);assertEquals("",d.values().get("weight"));assertTrue(d.undo());assertEquals("1",d.values().get("weight"));}
    @Test public void ambiguousApprovalAndUnknownUnitsDoNotExecute(){AgentDraft d=new AgentDraft();assertThrows(IllegalArgumentException.class,()->d.propose("はい",0,100));assertThrows(IllegalArgumentException.class,()->d.propose("0.07",0,100));assertThrows(IllegalStateException.class,()->d.accept(d.propose("モモ",0,100),30101));}
}

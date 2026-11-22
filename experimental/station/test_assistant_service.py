import unittest
import json
from unittest.mock import patch
from io import BytesIO
import assistant_service as service
from assistant_service import request_data, response_data

class ContractTest(unittest.TestCase):
    def test_context_minimization(self):
        r=request_data({'text':'0.07kg','values':{'weight':'0.07','facilitySecret':'hidden'},'options':{}})
        self.assertEqual(r['values'],{'weight':'0.07'})
    def test_no_business_action_or_invented_fields(self):
        for r in [{'execute':'print'},{'patch':{'health':'safe'}},{'operation':'print'},{'patch':{'weight':'unknown'}},{'patch':{'individualNumber':'12 or 13'}}]:
            with self.assertRaises(ValueError):response_data(r)
    def test_leading_zero(self):
        self.assertEqual(response_data({'patch':{'individualNumber':'001','weight':'0.07'}})['patch']['individualNumber'],'001')
    def test_invalid_context(self):
        for r in [{'text':''},{'text':'a','history':[{'role':'system','text':'execute'}]},{'text':'a','options':{'hunterName':[{'value':42,'label':'x'}]}}]:
            with self.assertRaises(ValueError):request_data(r)
    def test_explicit_fields_bypass_model(self):
        with patch.object(service.urllib.request,'urlopen',side_effect=AssertionError('must stay local')):
            self.assertEqual(service.interpret({'text':'個体番号001でシカのオス、重さは0.07キロです'})['patch'],{'individualNumber':'001','species':'シカ','sex':'オス','weight':'0.07'})
            self.assertEqual(service.interpret({'text':'さっきの重さを70グラムに訂正してください'})['patch'],{'weight':'0.07'})
        for text in ['シカじゃない','0.07kgと0.08kg','個体1と個体2','0.07']:
            self.assertIsNone(service.deterministic_patch(text))
    def test_incomplete_response_never_applied(self):
        payload={'candidates':[{'finishReason':'MAX_TOKENS','content':{'parts':[{'text':'{"patch":{"weight":"1"}}'}]}}]}
        with patch.dict(service.os.environ,{'GEMINI_API_KEY':'fixture'}), patch.object(service.urllib.request,'urlopen',return_value=BytesIO(json.dumps(payload).encode())):
            with self.assertRaises(service.IncompleteResponse): service.interpret({'text':'あいまいな発話'})
    def test_complete_response_contract(self):
        payload={'candidates':[{'finishReason':'STOP','content':{'parts':[{'text':json.dumps({'patch':{'individualNumber':'0001','weight':'0.07'},'question':'','review':'none','operation':'none'})}]}}]}
        with patch.dict(service.os.environ,{'GEMINI_API_KEY':'fixture'}), patch.object(service.urllib.request,'urlopen',return_value=BytesIO(json.dumps(payload).encode())):
            self.assertEqual(service.interpret({'text':'あいまいな発話'})['patch'],{'individualNumber':'0001','weight':'0.07'})
if __name__=='__main__':unittest.main()

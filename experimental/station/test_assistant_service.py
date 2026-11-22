import unittest
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
if __name__=='__main__':unittest.main()

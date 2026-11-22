import json
from pathlib import Path
import tempfile
import threading
import unittest
import urllib.error
import urllib.request
from secret_card import make_server, save_key


class SecretCardTest(unittest.TestCase):
    def test_trim_and_long_dotted_key(self):
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp)/"gemini.env"
            key="AQ."+"a"*500+"-_=+/."
            save_key(path,"\r\n \ufeff"+key+"\u200b \r\n")
            self.assertEqual(path.read_text(),"GEMINI_API_KEY="+key+"\n")
    def test_one_use_origin_and_permissions(self):
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp)/"secrets/gemini.env"
            server=make_server(0,"https://test.ts.net",path,"access-token")
            worker=threading.Thread(target=server.serve_forever,daemon=True);worker.start()
            try:
                def post(origin,token,value):
                    request=urllib.request.Request(f"http://127.0.0.1:{server.server_port}/api/key",data=json.dumps({"key":value}).encode(),headers={"Origin":origin,"Authorization":"Bearer "+token,"Content-Type":"application/json"})
                    return urllib.request.urlopen(request)
                for origin,token in [("https://evil.test","access-token"),("https://test.ts.net","wrong")]:
                    with self.assertRaises(urllib.error.HTTPError) as error:post(origin,token,"x"*39)
                    self.assertEqual(error.exception.code,403)
                self.assertFalse(path.exists())
                with self.assertRaises(urllib.error.HTTPError):post("https://test.ts.net","access-token","x"*30+"\nOTHER=x")
                with post("https://test.ts.net","access-token","x"*39) as result:self.assertEqual(json.load(result),{"saved":True})
                self.assertEqual(path.stat().st_mode&0o777,0o600)
                with self.assertRaises(urllib.error.HTTPError) as error:post("https://test.ts.net","access-token","y"*39)
                self.assertEqual(error.exception.code,409)
                self.assertEqual(path.read_text(),"GEMINI_API_KEY="+"x"*39+"\n")
            finally:server.shutdown();server.server_close();worker.join()


if __name__=="__main__":unittest.main()

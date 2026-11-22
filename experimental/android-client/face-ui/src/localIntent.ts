/** Deterministic, deliberately narrow. Ambiguous text goes to clarification, never execution. */
export function localIntent(text:string):any {
  const s=text.normalize('NFKC').replace(/[。!！\s]/g,'');
  if(/^(はい|いいね|もういい|確定)$/.test(s))return {question:'入力内容の確認、会話の終了、どちらにしますか？'};
  if(/^(計量|量って|計って|重量を確認)$/.test(s))return {operation:'weigh'};
  if(/^(スキャン|コードを読む)$/.test(s))return {operation:'scan'};
  if(/^(ラベル|ラベル印刷)$/.test(s))return {operation:'labels'};
  if(/^(確認|入力内容|入力済み)$/.test(s))return {review:'filled'};
  if(/^(未入力|残りの項目)$/.test(s))return {review:'missing'};
  if(/^(一覧|全部の項目)$/.test(s))return {review:'all'};
  if(/^(業務に戻る|業務画面)$/.test(s))return {navigate:''};
  if(/^(違う|ではない|じゃない|以外|かも|多分|たぶん)/.test(s)||/ではなく|じゃなく|ではない|かもしれ|それとも|または/.test(s))return null;
  const patch:Record<string,string>={};let conflict=false;
  const put=(key:string,value:string)=>{if(key in patch&&patch[key]!==value)conflict=true;patch[key]=value;return '';};
  const rest=s.replace(/[、,]/g,'')
    .replace(/(?:個体番号|個体)(?:は)?(\d{1,20})/g,(_,id)=>put('individualNumber',id))
    .replace(/(?:重量|重さ)?(?:は)?(\d+(?:\.\d+)?)(キログラム|キロ|kg|グラム|g)/gi,(_,value,unit)=>put('weight',String(Number(value)*(/^(g|グラム)$/i.test(unit)?.001:1))))
    .replace(/シカ|クマ|イノシシ|アライグマ/g,v=>put('species',v))
    .replace(/オス|メス/g,v=>put('sex',v));
  return !conflict&&Object.keys(patch).length&&/^(?:の|は|で|と|です|に訂正|に変更)*$/.test(rest)?{patch}:null;
}

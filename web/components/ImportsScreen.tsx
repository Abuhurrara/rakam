"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { confirmImport, createImportPairingCode, ignoreImport, listImportDevices, listImports, revokeImportDevice } from "@/lib/api";
import { formatPaisa, isValidAmountInput, paisaToInputString } from "@/lib/money";
import type { ImportDevice, ImportSuggestion, TransactionInput } from "@/lib/types";
import { useCategories } from "./CategoriesProvider";
import { useFinanceData } from "./FinanceDataProvider";

function karachiInput(iso: string): string {
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone: "Asia/Karachi", year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? "";
  return `${value("year")}-${value("month")}-${value("day")}T${value("hour")}:${value("minute")}`;
}

export function ImportsScreen() {
  const { expense } = useCategories();
  const finance = useFinanceData();
  const [items, setItems] = useState<ImportSuggestion[]>([]);
  const [devices, setDevices] = useState<ImportDevice[]>([]);
  const [code, setCode] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const refresh = useCallback(() => setNonce((n) => n + 1), []);
  useEffect(() => {
    let alive = true;
    setRefreshing(true);
    Promise.all([listImports(), listImportDevices()])
      .then(([imports, paired]) => {
        if (!alive) return;
        setItems(imports.suggestions);
        setDevices(paired.devices);
        setError(null);
      })
      .catch(() => { if (alive) setError("Could not refresh bank imports. Try again."); })
      .finally(() => { if (alive) { setLoading(false); setRefreshing(false); } });
    return () => { alive = false; };
  }, [nonce]);

  useEffect(() => {
    const visible = () => { if (document.visibilityState === "visible") refresh(); };
    window.addEventListener("online", refresh);
    document.addEventListener("visibilitychange", visible);
    return () => { window.removeEventListener("online", refresh); document.removeEventListener("visibilitychange", visible); };
  }, [refresh]);

  const pair = async () => {
    setBusy("pair"); setError(null);
    try { setCode((await createImportPairingCode()).code); }
    catch { setError("Could not make a pairing code. Try again."); }
    finally { setBusy(null); }
  };
  const ignore = async (id: string) => {
    setBusy(id); setError(null);
    try { await ignoreImport(id); setItems((current) => current.filter((item) => item.id !== id)); }
    catch { setError("Could not ignore this alert. Try again."); }
    finally { setBusy(null); }
  };
  const confirm = async (id: string, input: TransactionInput) => {
    setBusy(id); setError(null);
    try {
      const saved = await confirmImport(id, input);
      setItems((current) => current.filter((item) => item.id !== id));
      finance.recordSaved(saved.transaction);
      refresh();
    } catch { setError("Could not save this alert. It is still waiting for review."); }
    finally { setBusy(null); }
  };
  const revoke = async (id: string) => {
    setBusy(id); setError(null);
    try { await revokeImportDevice(id); setDevices((current) => current.filter((device) => device.id !== id)); }
    catch { setError("Could not disconnect this phone. Try again."); }
    finally { setBusy(null); }
  };

  return <div className="px-4 pb-24 pt-6">
    <h1 className="text-2xl font-semibold text-ink">Bank imports</h1>
    <p className="mt-1 text-sm text-ink-soft">Meezan SMS purchases wait here until you review them. They do not count in Expenses or Budget before you save.</p>
    <section className="mt-6 rounded-2xl border border-line bg-paper-raised p-4">
      <h2 className="font-semibold text-ink">Connect your Android phone</h2>
      <p className="mt-1 text-sm text-ink-soft">Install the Rakam companion app, enter this one-time code there, then allow SMS alerts from 8079.</p>
      <button type="button" onClick={pair} disabled={busy !== null} className="mt-4 min-h-11 rounded-full bg-primary px-5 text-sm font-semibold text-primary-ink disabled:opacity-50">{busy === "pair" ? "Making code…" : code ? "New pairing code" : "Get pairing code"}</button>
      {code && <div className="mt-3 rounded-xl bg-paper-sunken p-3"><p className="text-xs text-ink-soft">Expires in 10 minutes · use once</p><p className="mt-1 break-all font-mono text-lg font-semibold tracking-wide text-ink select-all">{code}</p></div>}
      {devices.length > 0 && <div className="mt-4 space-y-2"><p className="text-xs font-semibold uppercase tracking-wider text-ink-faint">Connected phones</p>{devices.map((device) => <div key={device.id} className="flex items-center justify-between gap-3 text-sm"><span className="truncate text-ink">{device.name}</span><button type="button" disabled={busy !== null} onClick={() => revoke(device.id)} className="min-h-11 px-2 font-medium text-brick disabled:opacity-50">Disconnect</button></div>)}</div>}
    </section>
    <div className="mt-7 flex items-center justify-between"><h2 className="text-lg font-semibold text-ink">To review {items.length > 0 ? `(${items.length})` : ""}</h2><button type="button" onClick={refresh} disabled={refreshing} className="min-h-11 px-2 text-sm font-medium text-primary disabled:opacity-50">{refreshing ? "Checking…" : "Refresh"}</button></div>
    {error && <p role="alert" className="mt-3 rounded-xl bg-brick-tint p-3 text-sm text-brick">{error}</p>}
    {loading && <p role="status" className="mt-4 text-sm text-ink-soft">Loading alerts…</p>}
    {!loading && items.length === 0 && <p className="mt-4 rounded-2xl border border-line p-5 text-sm text-ink-soft">No alerts waiting. New matching SMS alerts will appear here after your phone syncs.</p>}
    <div className="mt-3 space-y-3">{items.map((item) => <ImportCard key={item.id} item={item} categories={expense} busy={busy === item.id} onIgnore={ignore} onConfirm={confirm} />)}</div>
  </div>;
}

function ImportCard({item,categories,busy,onIgnore,onConfirm}:{item:ImportSuggestion;categories:{id:string;name:string;icon:string}[];busy:boolean;onIgnore:(id:string)=>void;onConfirm:(id:string,input:TransactionInput)=>void}) {
  const [amount,setAmount]=useState(() => paisaToInputString(item.amount_paisa));
  const [categoryID,setCategoryID]=useState("");
  const [description,setDescription]=useState(item.description);
  const [when,setWhen]=useState(() => karachiInput(item.occurred_at));
  const submit=(event:FormEvent<HTMLFormElement>)=>{
    event.preventDefault();
    if (!isValidAmountInput(amount) || !when) return;
    onConfirm(item.id,{kind:"expense",amount:amount.trim(),category_id:categoryID || null,description:description.trim() || null,occurred_at:`${when}:00+05:00`});
  };
  return <form onSubmit={submit} className="rounded-2xl border border-line bg-paper-raised p-4">
    <div className="flex items-start justify-between gap-3"><div><p className="text-xs font-medium uppercase tracking-wider text-ink-faint">Meezan card alert</p><p className="mt-1 text-lg font-semibold text-ink">{formatPaisa(item.amount_paisa)}</p></div><time className="text-xs text-ink-soft" dateTime={item.occurred_at}>{new Date(item.occurred_at).toLocaleDateString("en-PK",{timeZone:"Asia/Karachi",day:"numeric",month:"short"})}</time></div>
    <p className="mt-1 text-xs text-ink-soft">No merchant in this SMS. Check for an expense you already entered before saving.</p>
    <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2"><label className="text-xs font-medium text-ink-soft">Amount (Rs)<input inputMode="decimal" value={amount} onChange={(e)=>setAmount(e.target.value)} className="mt-1 min-h-11 w-full rounded-xl border border-line bg-paper px-3 text-base text-ink" /></label><label className="text-xs font-medium text-ink-soft">Date and time<input type="datetime-local" value={when} onChange={(e)=>setWhen(e.target.value)} className="mt-1 min-h-11 w-full rounded-xl border border-line bg-paper px-2 text-base text-ink" /></label></div>
    <label className="mt-3 block text-xs font-medium text-ink-soft">Category<select value={categoryID} onChange={(e)=>setCategoryID(e.target.value)} className="mt-1 min-h-11 w-full rounded-xl border border-line bg-paper px-3 text-sm text-ink"><option value="">No category yet</option>{categories.map((category)=><option key={category.id} value={category.id}>{category.icon} {category.name}</option>)}</select></label>
    <label className="mt-3 block text-xs font-medium text-ink-soft">Note<input value={description} maxLength={200} onChange={(e)=>setDescription(e.target.value)} className="mt-1 min-h-11 w-full rounded-xl border border-line bg-paper px-3 text-sm text-ink" /></label>
    <div className="mt-4 flex gap-2"><button type="submit" disabled={busy || !isValidAmountInput(amount) || !when} className="min-h-11 flex-1 rounded-full bg-primary px-4 text-sm font-semibold text-primary-ink disabled:opacity-50">{busy ? "Saving…" : "Add expense"}</button><button type="button" disabled={busy} onClick={()=>onIgnore(item.id)} className="min-h-11 rounded-full border border-line px-4 text-sm font-medium text-ink-soft disabled:opacity-50">Ignore</button></div>
  </form>;
}

import { ParseDir, BrowseDir } from "/wailsjs/go/app/App.js";

const q = s => document.querySelector(s);
const listEl = q('#list');
const metaEl = q('#meta');
const scrollerEl = q('#scroller');
const optionsEl = q('#options');

let trainers = [];
let selected = -1;

q('#scan').addEventListener('click', async () => {
  try {
    let dir = (q('#dir').value || '').trim();

    // If no path typed, open native directory picker
    if (!dir) {
      const chosen = await BrowseDir('.');
      if (!chosen) return;          // user canceled
      q('#dir').value = chosen;
      dir = chosen;
    }

    trainers = await ParseDir(dir);
    renderList();
    if (trainers.length) select(0);
    else clearDetail();
  } catch (e) {
    console.error(e);
    alert('Scan failed: ' + e);
  }
});

function renderList() {
  listEl.innerHTML = '';
  trainers.forEach((t, i) => {
    const item = document.createElement('div');
    item.className = 'item' + (i===selected ? ' active' : '');

    const img = document.createElement('img');
    img.src = t.iconURL || '';
    img.onerror = () => { img.src = ''; };

    const title = document.createElement('div');
    title.className = 'title';
    title.textContent = t.name || 'Trainer';

    const sub = document.createElement('div');
    sub.className = 'sub';
    sub.textContent = `${t.isXBTF ? 'XBTF' : 'ETM'} • ${base(t.path)}`;

    const wrap = document.createElement('div');
    wrap.appendChild(title);
    wrap.appendChild(sub);

    item.appendChild(img);
    item.appendChild(wrap);

    item.addEventListener('click', () => select(i));
    listEl.appendChild(item);
  });
}

function select(i) {
  selected = i;
  [...listEl.children].forEach((el, idx) => el.classList.toggle('active', idx === selected));
  renderDetail(trainers[i]);
}

function clearDetail() {
  metaEl.innerHTML = '';
  scrollerEl.textContent = '(none)';
  optionsEl.textContent = '(none)';
}

function renderDetail(t) {
  metaEl.innerHTML = '';
  const img = document.createElement('img');
  img.className = 'metaimg';
  img.src = t.iconURL || '';
  img.onerror = () => { img.src = ''; };

  const right = document.createElement('div');
  right.innerHTML = `
    <div class="kv"><div class="key">Name</div><div>${escapeHTML(t.name || 'Trainer')}</div></div>
    <div class="kv"><div class="key">Format</div><div>${t.isXBTF ? 'XBTF' : 'ETM'}</div></div>
    <div class="kv"><div class="key">CreationKey</div><div>${escapeHTML(t.creationKey || '(none)')}</div></div>
    <div class="kv"><div class="key">TitleIDs</div><div>${hex(t.titleIDs[0])} ${hex(t.titleIDs[1])} ${hex(t.titleIDs[2])}</div></div>
    <div class="kv"><div class="key">Offsets</div><div>Opt=0x${t.optStart.toString(16).toUpperCase()}  Text=0x${t.textStart.toString(16).toUpperCase()}</div></div>
  `;

  metaEl.appendChild(img);
  metaEl.appendChild(right);

  scrollerEl.textContent = (t.scroller && t.scroller.trim()) ? t.scroller : '(none)';
  const opts = (t.labels || []).filter(Boolean).map((v, i) => `${(i+1).toString().padStart(2,' ')}) ${v}`);
  optionsEl.textContent = opts.length ? opts.join('\n') : '(none)';
}

function hex(n) {
  if (typeof n !== 'number') return '00000000';
  let s = n >>> 0; // uint32
  let h = s.toString(16).toUpperCase();
  return ('00000000'+h).slice(-8);
}
function base(p){ return p.split(/[\\/]/).pop(); }
function escapeHTML(s){
  return String(s).replace(/[&<>"']/g, c => ({
    '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'
  }[c]));
}


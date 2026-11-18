import { useId } from 'react';
// Ảnh minh họa dạng SVG vẽ tay cho website khách hàng (dùng khi chưa có ảnh thật).
// Dùng đúng bảng màu của thương hiệu: navy #172554, xanh #2563EB, trắng và xanh nhạt;
// điểm nhấn ấm (ánh đèn) dùng màu Warning #F59E0B. Mỗi scene vẽ trong khung 400x300 (trừ hero).
const C = { n9: '#0f1a3d', n8: '#172554', n7: '#1e3a8a', b6: '#2563eb', b4: '#60a5fa', b3: '#93c5fd', b2: '#bfdbfe', b1: '#dbeafe', b0: '#eff6ff', w: '#ffffff', gold: '#F59E0B', glow: '#fde68a' };

// Số giả ngẫu nhiên ổn định (để ô cửa sổ sáng đèn không đổi giữa các lần render).
const hash = (a, b, seed = 1) => (((a * 73856093) ^ (b * 19349663) ^ (seed * 83492791)) >>> 0) % 100 / 100;

function Frame({ id, viewBox = '0 0 400 300', defs, children }) {
  return (
    <svg
      viewBox={viewBox}
      preserveAspectRatio="xMidYMid slice"
      className="c-zoom absolute inset-0 h-full w-full"
      aria-hidden="true"
      focusable="false"
      data-scene={id}
    >
      {defs && <defs>{defs}</defs>}
      {children}
    </svg>
  );
}

const Grad = ({ id, stops, x2 = 0, y2 = 1 }) => (
  <linearGradient id={id} x1="0" y1="0" x2={x2} y2={y2}>
    {stops.map(([o, c, op]) => (
      <stop key={o} offset={o} stopColor={c} stopOpacity={op ?? 1} />
    ))}
  </linearGradient>
);

function Buildings({ list, baseY, fill, opacity = 1, windows = false, seed = 1 }) {
  return (
    <g opacity={opacity}>
      {list.map(([x, w, h], i) => (
        <g key={i}>
          <rect x={x} y={baseY - h} width={w} height={h} fill={fill} />
          {windows &&
            Array.from({ length: Math.floor((h - 10) / 14) }, (_, r) =>
              Array.from({ length: Math.floor((w - 6) / 10) }, (_, c) => {
                const lit = hash(i * 31 + c, r, seed) > 0.58;
                return (
                  <rect key={`${r}-${c}`} x={x + 5 + c * 10} y={baseY - h + 8 + r * 14} width="5" height="7" fill={lit ? C.gold : C.n7} opacity={lit ? 0.9 : 0.55} />
                );
              }),
            )}
        </g>
      ))}
    </g>
  );
}

/* ---------------- Phòng ngủ ---------------- */
const SKY_CITY = [[262, 16, 44], [276, 13, 70], [288, 18, 36], [304, 14, 60], [316, 20, 46], [330, 12, 54]];

function Bed({ x, w }) {
  const pw = (w - 20) / 2;
  return (
    <g>
      <rect x={x} y="120" width={w} height="70" rx="10" fill={C.b6} />
      <rect x={x + 8} y="128" width={w - 16} height="54" rx="6" fill={C.n7} opacity=".6" />
      <rect x={x - 6} y="176" width={w + 12} height="48" rx="10" fill={C.w} />
      <rect x={x + 8} y="162" width={pw} height="24" rx="9" fill={C.b0} stroke={C.b2} />
      <rect x={x + 12 + pw} y="162" width={pw} height="24" rx="9" fill={C.b0} stroke={C.b2} />
      <rect x={x - 6} y="192" width={w + 12} height="32" rx="8" fill={C.b6} />
      <rect x={x - 6} y="192" width={w + 12} height="9" rx="4" fill={C.b4} />
      <rect x={x - 10} y="222" width={w + 20} height="10" rx="3" fill={C.n8} />
    </g>
  );
}

function RoomScene({ beds = 1, sofa = false, balcony = false, desk = false, art = false, tag }) {
  const u = useId().replace(/:/g, '');
  return (
    <Frame
      id={tag}
      viewBox="24 0 352 300"
      defs={
        <>
          <Grad id={`${u}rs-wall`} stops={[[0, C.n9], [1, C.n7]]} />
          <Grad id={`${u}rs-sky`} stops={[[0, C.n8], [0.55, C.b6], [1, C.glow]]} />
          <radialGradient id={`${u}rs-lamp`}><stop offset="0" stopColor={C.glow} stopOpacity=".75" /><stop offset="1" stopColor={C.glow} stopOpacity="0" /></radialGradient>
          <clipPath id={`${u}rs-win`}><path d="M258 216V118a42 42 0 0 1 84 0V216Z" /></clipPath>
        </>
      }
    >
      <rect width="400" height="300" fill={`url(#${u}rs-wall)`} />
      <rect y="228" width="400" height="72" fill={C.n9} />
      <rect y="228" width="400" height="4" fill={C.b4} opacity=".5" />
      <ellipse cx="140" cy="268" rx="124" ry="14" fill={C.b6} opacity=".45" />
      {art && <g><rect x="84" y="52" width="74" height="48" rx="3" fill={C.b6} stroke={C.b3} strokeWidth="3" /><circle cx="112" cy="74" r="8" fill={C.glow} /><path d="M84 100l22-20 16 12 14-10 22 18z" fill={C.n8} /></g>}
      {/* cửa sổ vòm */}
      <g clipPath={`url(#${u}rs-win)`}>
        <rect x="258" y="70" width="84" height="150" fill={`url(#${u}rs-sky)`} />
        <circle cx="318" cy="130" r="9" fill={C.glow} opacity=".9" />
        {SKY_CITY.map(([x, w, h], i) => <rect key={i} x={x + 4} y={216 - h} width={w} height={h} fill={C.n7} opacity={0.35 + (i % 3) * 0.15} />)}
      </g>
      <path d="M258 216V118a42 42 0 0 1 84 0V216Z" fill="none" stroke={C.w} strokeWidth="6" />
      <path d="M300 76v140M258 160h84" stroke={C.w} strokeWidth="3" />
      {balcony && (
        <g stroke={C.b3} strokeWidth="2.5">
          <path d="M250 200h100" />
          {Array.from({ length: 11 }, (_, i) => <path key={i} d={`M${254 + i * 9.4} 200v26`} />)}
        </g>
      )}
      {beds === 1 ? <Bed x="44" w="164" /> : <><Bed x="26" w="94" /><Bed x="134" w="94" /></>}
      {/* đèn ngủ */}
      <circle cx="236" cy="196" r="46" fill={`url(#${u}rs-lamp)`} />
      <rect x="226" y="206" width="22" height="24" rx="3" fill={C.b6} />
      <path d="M228 198l4-14h12l4 14z" fill={C.gold} />
      {sofa && (
        <g>
          <rect x="286" y="190" width="104" height="38" rx="10" fill={C.b6} />
          <rect x="280" y="176" width="14" height="52" rx="6" fill={C.b4} />
          <rect x="382" y="176" width="14" height="52" rx="6" fill={C.b4} />
          <rect x="298" y="180" width="38" height="26" rx="8" fill={C.b3} />
        </g>
      )}
      {desk && (
        <g>
          <rect x="300" y="196" width="84" height="7" rx="2" fill={C.b4} />
          <path d="M306 203v26M378 203v26" stroke={C.b4} strokeWidth="5" />
          <rect x="324" y="176" width="30" height="20" rx="2" fill={C.b2} />
        </g>
      )}
    </Frame>
  );
}

function BathScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame id="bath" defs={<Grad id={`${u}bs-wall`} stops={[[0, C.b0], [1, C.b2]]} />}>
      <rect width="400" height="300" fill={`url(#${u}bs-wall)`} />
      <g stroke={C.w} strokeWidth="2" opacity=".7">
        {Array.from({ length: 11 }, (_, i) => <path key={`v${i}`} d={`M${i * 40} 0v230`} />)}
        {Array.from({ length: 7 }, (_, i) => <path key={`h${i}`} d={`M0 ${i * 38}h400`} />)}
      </g>
      <rect y="228" width="400" height="72" fill={C.n7} />
      <path d="M300 176V92a28 28 0 0 1 56 0v84z" fill={C.b3} stroke={C.w} strokeWidth="6" />
      <path d="M312 150c8-26 20-30 32-30" stroke={C.w} strokeWidth="3" fill="none" opacity=".7" />
      <rect x="44" y="164" width="230" height="64" rx="32" fill={C.w} stroke={C.b2} strokeWidth="3" />
      <rect x="62" y="176" width="194" height="14" rx="7" fill={C.b2} opacity=".8" />
      <path d="M66 228v12M252 228v12" stroke={C.n9} strokeWidth="6" strokeLinecap="round" />
      <path d="M240 164v-34a14 14 0 0 1 28 0" stroke={C.n8} strokeWidth="5" fill="none" strokeLinecap="round" />
      <path d="M84 70h96" stroke={C.n8} strokeWidth="4" strokeLinecap="round" />
      <rect x="96" y="70" width="34" height="78" rx="4" fill={C.b6} />
      <rect x="138" y="70" width="34" height="62" rx="4" fill={C.w} stroke={C.b2} />
      <circle cx="342" cy="206" r="14" fill={C.b6} opacity=".5" />
    </Frame>
  );
}

function ViewScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame
      id="view"
      defs={
        <>
          <Grad id={`${u}vs-sky`} stops={[[0, C.n8], [0.55, C.b6], [1, C.b2]]} />
          <radialGradient id={`${u}vs-sun`}><stop offset="0" stopColor={C.glow} stopOpacity=".9" /><stop offset="1" stopColor={C.glow} stopOpacity="0" /></radialGradient>
        </>
      }
    >
      <rect width="400" height="300" fill={`url(#${u}vs-sky)`} />
      <circle cx="270" cy="170" r="90" fill={`url(#${u}vs-sun)`} />
      <circle cx="270" cy="170" r="22" fill={C.glow} />
      <Buildings baseY={244} fill={C.n7} opacity=".55" seed={3} list={[[0, 40, 90], [44, 30, 130], [78, 44, 100], [126, 28, 150], [158, 40, 110], [204, 34, 80], [244, 44, 140], [292, 30, 96], [326, 46, 120], [376, 30, 84]]} />
      <Buildings baseY={252} fill={C.n9} windows seed={5} list={[[10, 52, 120], [70, 38, 170], [114, 60, 100], [182, 44, 150], [232, 56, 124], [294, 40, 180], [340, 56, 110]]} />
      <rect y="244" width="400" height="56" fill={C.n8} />
      <g stroke={C.w} strokeWidth="3" opacity=".9">
        <path d="M0 252h400" />
        {Array.from({ length: 22 }, (_, i) => <path key={i} d={`M${10 + i * 18.5} 252v48`} />)}
      </g>
    </Frame>
  );
}

function PoolScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame id="pool" defs={<><Grad id={`${u}ps-sky`} stops={[[0, C.b2], [1, C.b0]]} /><Grad id={`${u}ps-water`} stops={[[0, C.b4], [1, C.b6]]} /></>}>
      <rect width="400" height="300" fill={`url(#${u}ps-sky)`} />
      <circle cx="318" cy="70" r="30" fill={C.glow} />
      <circle cx="318" cy="70" r="46" fill={C.glow} opacity=".25" />
      <Buildings baseY={150} fill={C.n7} opacity=".45" list={[[0, 48, 70], [52, 36, 100], [92, 54, 60], [300, 44, 84], [348, 52, 62]]} />
      <rect y="150" width="400" height="150" fill={`url(#${u}ps-water)`} />
      <g stroke={C.w} strokeWidth="3" strokeLinecap="round" fill="none" opacity=".55">
        <path d="M20 176c20-8 40 8 60 0s40 8 60 0 40 8 60 0 40 8 60 0 40 8 60 0" />
        <path d="M0 204c20-8 40 8 60 0s40 8 60 0 40 8 60 0 40 8 60 0 40 8 60 0 40 8 40 0" />
      </g>
      <rect y="236" width="400" height="64" fill={C.n7} />
      <rect y="232" width="400" height="8" fill={C.w} />
      <g>
        <path d="M60 262l40-14v10l-40 14z" fill={C.w} />
        <path d="M60 262v12M96 258v12" stroke={C.n9} strokeWidth="4" />
        <path d="M132 262l40-14v10l-40 14z" fill={C.w} />
        <path d="M132 262v12M168 258v12" stroke={C.n9} strokeWidth="4" />
        <path d="M248 206v56" stroke={C.n9} strokeWidth="4" />
        <path d="M210 214a38 24 0 0 1 76 0z" fill={C.b6} />
        <path d="M229 214a19 24 0 0 1 38 0z" fill={C.w} opacity=".85" />
      </g>
    </Frame>
  );
}

function RestaurantScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame
      id="restaurant"
      defs={
        <>
          <Grad id={`${u}rt-wall`} stops={[[0, C.n8], [1, C.n7]]} />
          <radialGradient id={`${u}rt-glow`}><stop offset="0" stopColor={C.glow} stopOpacity=".7" /><stop offset="1" stopColor={C.glow} stopOpacity="0" /></radialGradient>
        </>
      }
    >
      <rect width="400" height="300" fill={`url(#${u}rt-wall)`} />
      {[70, 200, 330].map((x) => (
        <path key={x} d={`M${x - 36} 200V104a36 36 0 0 1 72 0V200Z`} fill={C.b4} opacity=".35" stroke={C.b3} strokeWidth="3" />
      ))}
      {[90, 200, 310].map((x, i) => (
        <g key={x}>
          <path d={`M${x} 0v${40 + (i % 2) * 14}`} stroke={C.b3} strokeWidth="2" />
          <circle cx={x} cy={64 + (i % 2) * 14} r="46" fill={`url(#${u}rt-glow)`} />
          <path d={`M${x - 16} ${62 + (i % 2) * 14}a16 16 0 0 1 32 0z`} fill={C.gold} />
        </g>
      ))}
      <rect y="228" width="400" height="72" fill={C.n9} />
      {[88, 200, 312].map((x) => (
        <g key={x}>
          <ellipse cx={x} cy="238" rx="46" ry="12" fill={C.w} />
          <path d={`M${x} 248v34M${x - 22} 284h44`} stroke={C.b3} strokeWidth="5" strokeLinecap="round" />
          <ellipse cx={x} cy="235" rx="15" ry="4" fill={C.b2} />
          <rect x={x + 18} y="222" width="5" height="14" rx="2" fill={C.b4} />
        </g>
      ))}
    </Frame>
  );
}

function SpaScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame id="spa" defs={<><Grad id={`${u}sp-bg`} stops={[[0, C.b0], [1, C.b2]]} /><radialGradient id={`${u}sp-glow`}><stop offset="0" stopColor={C.glow} stopOpacity=".8" /><stop offset="1" stopColor={C.glow} stopOpacity="0" /></radialGradient></>}>
      <rect width="400" height="300" fill={`url(#${u}sp-bg)`} />
      <circle cx="320" cy="90" r="64" fill={C.w} opacity=".5" />
      <rect y="222" width="400" height="78" fill={C.n7} />
      <ellipse cx="170" cy="236" rx="120" ry="14" fill={C.n8} opacity=".7" />
      <ellipse cx="170" cy="228" rx="64" ry="16" fill={C.b3} />
      <ellipse cx="170" cy="204" rx="48" ry="14" fill={C.b2} />
      <ellipse cx="170" cy="184" rx="32" ry="11" fill={C.w} />
      <ellipse cx="170" cy="168" rx="18" ry="8" fill={C.b1} />
      <g>
        <circle cx="300" cy="190" r="34" fill={`url(#${u}sp-glow)`} />
        <rect x="288" y="196" width="24" height="32" rx="4" fill={C.w} />
        <path d="M300 172c8 8 6 16 0 20-6-4-8-12 0-20z" fill={C.gold} />
      </g>
      <g fill={C.b6} opacity=".7">
        <path d="M60 222c-10-30-4-60 18-84 12 32 4 62-18 84z" />
        <path d="M60 222c8-28 28-48 52-56-2 30-22 50-52 56z" fill={C.b4} />
      </g>
      <rect x="338" y="212" width="46" height="16" rx="8" fill={C.w} />
      <rect x="338" y="198" width="46" height="16" rx="8" fill={C.b2} />
    </Frame>
  );
}

function LobbyScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame
      id="lobby"
      defs={<><Grad id={`${u}lb-wall`} stops={[[0, C.n8], [1, C.n7]]} /><radialGradient id={`${u}lb-glow`}><stop offset="0" stopColor={C.glow} stopOpacity=".75" /><stop offset="1" stopColor={C.glow} stopOpacity="0" /></radialGradient></>}
    >
      <rect width="400" height="300" fill={`url(#${u}lb-wall)`} />
      {[56, 150, 250, 344].map((x) => (
        <path key={x} d={`M${x - 36} 232V96a36 36 0 0 1 72 0V232Z`} fill={C.n9} stroke={C.b4} strokeWidth="3" opacity=".9" />
      ))}
      {[56, 150, 250, 344].map((x) => <path key={x} d={`M${x} 60v172`} stroke={C.b4} strokeWidth="1.5" opacity=".35" />)}
      <circle cx="200" cy="40" r="64" fill={`url(#${u}lb-glow)`} />
      <path d="M200 0v22" stroke={C.b3} strokeWidth="2" />
      <g fill={C.gold}>
        {[-26, -13, 0, 13, 26].map((d) => <circle key={d} cx={200 + d} cy={32 + Math.abs(d) * 0.35} r="4.5" />)}
      </g>
      <path d="M170 28h60l-8 18h-44z" fill={C.b4} opacity=".5" />
      <rect y="232" width="400" height="68" fill={C.n9} />
      <rect y="232" width="400" height="4" fill={C.b4} opacity=".5" />
      <rect x="120" y="196" width="160" height="40" rx="6" fill={C.b6} />
      <rect x="120" y="196" width="160" height="8" rx="4" fill={C.b3} />
      <ellipse cx="200" cy="266" rx="120" ry="12" fill={C.b6} opacity=".25" />
    </Frame>
  );
}

/* ---------------- Hero (khung dọc 480x640) ---------------- */
const BACK = [[0, 54, 150], [58, 40, 210], [102, 56, 130], [162, 36, 190], [202, 60, 160], [266, 40, 240], [310, 56, 140], [370, 40, 200], [414, 66, 120]];
const FRONT = [[8, 60, 230], [74, 48, 320], [130, 70, 200], [286, 56, 300], [346, 70, 220], [420, 56, 280]];

function HeroScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame
      id="hero"
      viewBox="0 0 480 640"
      defs={
        <>
          <Grad id={`${u}hs-sky`} stops={[[0, C.n9], [0.34, C.n7], [0.62, C.b6], [0.86, C.b3]]} />
          <radialGradient id={`${u}hs-sun`}><stop offset="0" stopColor={C.glow} stopOpacity=".95" /><stop offset=".4" stopColor={C.glow} stopOpacity=".28" /><stop offset="1" stopColor={C.glow} stopOpacity="0" /></radialGradient>
          <Grad id={`${u}hs-water`} stops={[[0, C.n8], [1, C.n9]]} />
        </>
      }
    >
      <rect width="480" height="640" fill={`url(#${u}hs-sky)`} />
      {Array.from({ length: 34 }, (_, i) => (
        <circle key={i} cx={hash(i, 1, 9) * 480} cy={hash(i, 2, 9) * 230} r={0.8 + hash(i, 3, 9) * 1.2} fill={C.w} opacity={0.35 + hash(i, 4, 9) * 0.5} />
      ))}
      <g className="hero-sun">
        <circle cx="330" cy="392" r="150" fill={`url(#${u}hs-sun)`} />
        <circle cx="330" cy="392" r="40" fill={C.glow} />
      </g>
      <g fill={C.w} opacity=".13">
        <ellipse cx="120" cy="300" rx="90" ry="12" />
        <ellipse cx="190" cy="318" rx="70" ry="9" />
        <ellipse cx="380" cy="250" rx="80" ry="10" />
      </g>
      <Buildings baseY={500} fill={C.n7} opacity=".6" seed={2} list={BACK} />
      <Buildings baseY={510} fill={C.n9} windows seed={7} list={FRONT} />
      {/* tòa khách sạn trung tâm */}
      <g>
        <rect x="196" y="190" width="82" height="324" fill={C.n8} />
        <rect x="190" y="182" width="94" height="14" fill={C.n7} />
        <rect x="236" y="148" width="2" height="36" fill={C.b3} />
        <circle cx="237" cy="146" r="4" fill={C.gold} />
        {Array.from({ length: 20 }, (_, r) =>
          Array.from({ length: 6 }, (_, c) => {
            const lit = hash(c, r, 11) > 0.4;
            return <rect key={`${r}-${c}`} x={204 + c * 12} y={206 + r * 15} width="7" height="9" fill={lit ? C.gold : C.n7} opacity={lit ? 0.92 : 0.6} />;
          }),
        )}
      </g>
      <rect y="510" width="480" height="130" fill={`url(#${u}hs-water)`} />
      <g stroke={C.gold} strokeWidth="2.5" strokeLinecap="round" opacity=".55">
        <path d="M214 528h44M222 544h28M228 560h16" />
      </g>
      <g stroke={C.b3} strokeWidth="2" strokeLinecap="round" opacity=".28">
        <path d="M30 540h110M300 548h150M70 580h120M260 596h170M20 620h90" />
      </g>
    </Frame>
  );
}

/* ---------------- Bản đồ minh họa ---------------- */
function MapScene() {
  const u = useId().replace(/:/g, '');
  return (
    <Frame id="map" viewBox="0 0 400 300" defs={<Grad id={`${u}mp-bg`} stops={[[0, C.b0], [1, C.b1]]} />}>
      <rect width="400" height="300" fill={`url(#${u}mp-bg)`} />
      <path d="M0 210c60-30 90 10 150-20s70-60 130-50 80 20 120-10v170H0z" fill={C.b2} opacity=".7" />
      {[[20, 20, 70, 50], [110, 20, 60, 70], [190, 30, 80, 40], [290, 20, 90, 60], [20, 90, 60, 50], [200, 100, 70, 60], [300, 110, 80, 50], [30, 170, 90, 50], [150, 200, 70, 60], [250, 190, 60, 70]].map(([x, y, w, h], i) => (
        <rect key={i} x={x} y={y} width={w} height={h} rx="6" fill={C.w} opacity=".85" />
      ))}
      <g stroke={C.w} strokeWidth="9" strokeLinecap="round" fill="none">
        <path d="M0 80h400M0 150h400M100 0v300M180 0v300M280 0v300" />
      </g>
      <g stroke={C.b3} strokeWidth="2" strokeDasharray="6 8" fill="none"><path d="M0 150h400M180 0v300" /></g>
      <path d="M180 150a0 0 0 0 0 0 0" />
      <g transform="translate(180 128)">
        <ellipse cx="0" cy="26" rx="14" ry="5" fill={C.n9} opacity=".25" />
        <path d="M0 24c-18-22-22-30-22-42a22 22 0 0 1 44 0c0 12-4 20-22 42z" fill={C.b6} stroke={C.w} strokeWidth="3" />
        <circle cx="0" cy="-18" r="8" fill={C.w} />
      </g>
    </Frame>
  );
}

export const SCENES = {
  map: MapScene,
  hero: HeroScene,
  lobby: LobbyScene,
  pool: PoolScene,
  restaurant: RestaurantScene,
  spa: SpaScene,
  bath: BathScene,
  view: ViewScene,
  standard: () => <RoomScene tag="standard" art />,
  superior: () => <RoomScene tag="superior" desk art />,
  deluxe: () => <RoomScene tag="deluxe" balcony />,
  family: () => <RoomScene tag="family" beds={2} />,
  suite: () => <RoomScene tag="suite" sofa balcony />,
};

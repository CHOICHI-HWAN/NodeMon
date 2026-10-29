export function Sparkline({ values, color }: { values: (number | null)[]; color: string }) {
  const W = 100, H = 28;
  const step = W / Math.max(1, values.length - 1);
  let d = "";
  let run = 0;
  values.forEach((v, i) => {
    if (v == null) {
      run = 0;
      return;
    }
    d += `${run++ === 0 ? "M" : "L"}${(i * step).toFixed(1)} ${(H - 2 - (Math.min(v, 100) / 100) * (H - 4)).toFixed(1)}`;
  });
  return (
    <svg className="spark" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-hidden="true">
      <path d={d} fill="none" stroke={color} strokeWidth={2} vectorEffect="non-scaling-stroke" strokeLinejoin="round" />
    </svg>
  );
}

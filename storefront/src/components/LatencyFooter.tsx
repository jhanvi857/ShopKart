import React from 'react';
import { LatencyMetric } from '../types';
import { Activity, CheckCircle, AlertTriangle, XCircle } from 'lucide-react';

interface LatencyFooterProps {
  lastMetric: LatencyMetric | null;
}

export const LatencyFooter: React.FC<LatencyFooterProps> = ({ lastMetric }) => {
  if (!lastMetric) {
    return (
      <footer className="fixed bottom-0 left-0 right-0 z-30 bg-[#212121] text-slate-300 border-t border-slate-700 text-[11px] py-1.5 px-4 shadow-sm select-none">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Activity className="w-3 h-3 text-[#388e3c]" />
            <span className="font-medium">ShopKart Core Services: Connected</span>
          </div>
          <span className="text-slate-400">Gateway Port 9000</span>
        </div>
      </footer>
    );
  }

  const isError = lastMetric.status >= 400 || !!lastMetric.error;
  const isSlow = !isError && lastMetric.durationMs > 350;

  let badgeColor = 'bg-[#388e3c]/20 text-[#388e3c] border-[#388e3c]/40';
  let Icon = CheckCircle;
  let label = 'Normal';

  if (isError) {
    badgeColor = 'bg-rose-500/20 text-rose-400 border-rose-500/40';
    Icon = XCircle;
    label = `HTTP ${lastMetric.status}`;
  } else if (isSlow) {
    badgeColor = 'bg-amber-500/20 text-amber-400 border-amber-500/40';
    Icon = AlertTriangle;
    label = 'Degraded';
  }

  return (
    <footer className="fixed bottom-0 left-0 right-0 z-30 bg-[#212121] text-slate-300 border-t border-slate-700 text-[11px] py-1.5 px-4 shadow-md select-none">
      <div className="max-w-7xl mx-auto flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-1.5 font-medium text-slate-200">
            <Activity className="w-3.5 h-3.5 text-slate-400" />
            <span>Edge Gateway:</span>
          </div>
          <span className="font-mono text-slate-400 truncate max-w-xs">{lastMetric.endpoint}</span>
        </div>

        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5">
            <span className="text-slate-400">Latency:</span>
            <span className={`font-mono font-bold ${isError ? 'text-rose-400' : isSlow ? 'text-amber-400' : 'text-[#388e3c]'}`}>
              {lastMetric.durationMs}ms
            </span>
          </div>

          <div className={`flex items-center gap-1 px-2 py-0.5 rounded border text-[10px] font-bold ${badgeColor}`}>
            <Icon className="w-3 h-3" />
            <span>{label}</span>
          </div>

          <span className="text-slate-500 text-[10px] hidden sm:inline">
            {lastMetric.timestamp}
          </span>
        </div>
      </div>
    </footer>
  );
};

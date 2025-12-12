'use client';

import React, { useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { agentAPI } from '@/lib/api';
import { Upload, Download, Loader2 } from 'lucide-react';

export interface UploadedFileInfo {
  name: string;
  absolutePath: string; // 服务器绝对路径
  size?: number;
}

interface FileTransferProps {
  enabled: boolean;
  onUploaded?: (info: UploadedFileInfo) => void;
}

export function FileTransfer({ enabled, onUploaded }: FileTransferProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [lastUploaded, setLastUploaded] = useState<UploadedFileInfo | null>(null);
  const [downloadName, setDownloadName] = useState('');

  const handleSelectClick = () => {
    if (!enabled) return;
    fileInputRef.current?.click();
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] || null;
    setSelectedFile(file);
  };

  const handleUpload = async () => {
    if (!enabled || !selectedFile || uploading) return;
    setUploading(true);
    try {
      const resp = await agentAPI.uploadFile(selectedFile);
      // 使用服务器返回的绝对路径（path 或 abs_path）
      const absolutePath = resp.path || resp.abs_path || '';
      const info: UploadedFileInfo = {
        name: resp.file || selectedFile.name,
        absolutePath,
        size: resp.size,
      };
      setLastUploaded(info);
      onUploaded?.(info);
    } catch (err) {
      console.error('Upload failed:', err);
      onUploaded?.({
        name: selectedFile.name,
        absolutePath: '',
      });
    } finally {
      setUploading(false);
    }
  };

  const handleDownload = async () => {
    const target = downloadName || lastUploaded?.name;
    if (!target) return;
    try {
      const blob = await agentAPI.downloadFile(target);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = target;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error('Download failed:', err);
    }
  };

  return (
    <div className="space-y-2">
      <input
        ref={fileInputRef}
        type="file"
        className="hidden"
        onChange={handleFileChange}
      />

      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleSelectClick}
          disabled={!enabled || uploading}
        >
          <Upload className="h-4 w-4 mr-2" />
          选择文件
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={handleUpload}
          disabled={!enabled || !selectedFile || uploading}
        >
          {uploading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            '上传'
          )}
        </Button>
        {selectedFile && (
          <span className="text-xs text-muted-foreground truncate">
            {selectedFile.name}
          </span>
        )}
      </div>

      <div className="flex items-center gap-2">
        <Label className="text-xs">下载:</Label>
        <Input
          value={downloadName}
          onChange={(e) => setDownloadName(e.target.value)}
          placeholder={lastUploaded?.name || '输入文件名'}
          className="h-8 text-xs w-48"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleDownload}
          disabled={uploading}
        >
          <Download className="h-4 w-4 mr-1" />
          下载
        </Button>
      </div>

      {lastUploaded && (
        <div className="text-xs text-muted-foreground">
          已上传: {lastUploaded.name}（绝对路径: {lastUploaded.absolutePath}）
        </div>
      )}
    </div>
  );
}


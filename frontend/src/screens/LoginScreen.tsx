export function LoginScreen({
    canvasRef,
    status,
  }: {
    canvasRef: React.RefObject<HTMLCanvasElement | null>;
    status?: string;
  }) {
    return (
        <div className="min-h-screen flex items-center justify-center bg-light-secondary dark:bg-dark-bg p-4">
          <div className="w-full max-w-[900px] h-[520px] bg-white dark:bg-gray-900 rounded-lg shadow-xl flex overflow-hidden border border-gray-100 dark:border-gray-800">
            {/* LEFT */}
            <div className="flex-1 p-12 flex flex-col justify-center">
              <div className="flex items-center gap-3 mb-8">
                <img src="https://upload.wikimedia.org/wikipedia/commons/thumb/6/6b/WhatsApp.svg/1022px-WhatsApp.svg.png" className="w-10 h-10" alt="WhatsApp Logo" />
                <h1 className="text-2xl font-light text-gray-700 dark:text-gray-200">Log in to WhatsApp</h1>
              </div>
    
              <ol className="list-decimal pl-6 space-y-4 text-gray-600 dark:text-gray-400 text-lg leading-relaxed">
                <li>Open WhatsApp on your phone</li>
                <li>Tap <b className="font-semibold text-gray-800 dark:text-gray-200">Menu</b> or <b className="font-semibold text-gray-800 dark:text-gray-200">Settings</b></li>
                <li>Select <b className="font-semibold text-gray-800 dark:text-gray-200">Linked Devices</b></li>
                <li>Point your phone at this screen</li>
              </ol>
            </div>
    
            {/* RIGHT */}
            <div className="flex-1 p-12 flex flex-col items-center justify-center border-l border-gray-100 dark:border-gray-800">
              <div className="bg-white p-2 rounded-lg shadow-sm mb-6">
                <canvas ref={canvasRef} className="w-[264px] h-[264px]" />
              </div>
    
              <p className="text-gray-500 dark:text-gray-400 text-center mb-4">
                Scan this QR code with the WhatsApp app
              </p>
    
              <p className="text-sm font-medium text-light-primary dark:text-green-400 animate-pulse uppercase tracking-wide">{status}</p>
            </div>
          </div>
        </div>
    );
  }
  
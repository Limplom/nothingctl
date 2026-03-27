"""Custom exceptions for the nothing firmware manager."""


class FirmwareError(Exception):       pass
class FlashError(Exception):          pass
class AdbError(Exception):            pass
class FastbootTimeoutError(Exception): pass
class MagiskError(Exception):         pass

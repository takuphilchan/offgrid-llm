"""Shared public client error, independent of transport and resource helpers."""


class OffGridError(Exception):
    """Base exception for OffGrid errors."""

    def __init__(self, message: str, code: str = None, details: str = None):
        self.message = message
        self.code = code
        self.details = details
        super().__init__(message)
